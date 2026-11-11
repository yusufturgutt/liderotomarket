// Lider Otomarket web sitesi.
//
//	liderotomarket serve  [-addr :8080]   siteyi yayınlar
//	liderotomarket sync   [-dry-run]      urunler.json'daki ürünleri data/ klasörüne işler
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"liderotomarket/internal/adapters/driven/imagestore"
	"liderotomarket/internal/adapters/driven/jsonfile"
	"liderotomarket/internal/adapters/driven/shopier"
	"liderotomarket/internal/adapters/driven/sqlite"
	"liderotomarket/internal/adapters/driving/cli"
	httpserver "liderotomarket/internal/adapters/driving/http"
	"liderotomarket/internal/config"
	"liderotomarket/internal/core/ports"
	"liderotomarket/internal/core/services/catalog"
	syncsvc "liderotomarket/internal/core/services/sync"
	"liderotomarket/web"
)

func usage() {
	fmt.Fprint(os.Stderr, `Kullanım:
  liderotomarket serve [-addr :8080]   Siteyi başlatır
  liderotomarket sync  [-dry-run]      urunler.json'daki ürünleri ve görselleri data/ klasörüne işler
                       [-kaynak shopier]  (Shopier ürün API'si açılırsa API'den çeker)

Ortak bayraklar: -config config.yaml -data data
`)
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var err error
	switch os.Args[1] {
	case "serve":
		err = serve(ctx, log, os.Args[2:])
	case "sync":
		err = runSync(ctx, log, os.Args[2:])
	case "-h", "--help", "help":
		usage()
		return
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "Hata:", err)
		os.Exit(1)
	}
}

type commonFlags struct {
	config, data string
}

func (c *commonFlags) register(fs *flag.FlagSet) {
	fs.StringVar(&c.config, "config", "config.yaml", "ayar dosyası")
	fs.StringVar(&c.data, "data", "data", "veri klasörü (catalog.db ve images/)")
}

func runSync(ctx context.Context, log *slog.Logger, args []string) error {
	fs := flag.NewFlagSet("sync", flag.ExitOnError)
	var c commonFlags
	c.register(fs)
	dry := fs.Bool("dry-run", false, "yalnızca değişiklikleri göster, kaydetme")
	kaynak := fs.String("kaynak", "urunler.json", "ürün kaynağı: JSON dosyasının yolu veya \"shopier\" (Shopier API)")
	timeout := fs.Duration("timeout", 15*time.Minute, "en uzun süre")
	fs.Parse(args)

	cfg, err := config.Load(c.config, ".env")
	if err != nil {
		return err
	}
	var source ports.ProductSource
	if *kaynak == "shopier" {
		api, err := shopier.New(cfg.ShopierToken, cfg.Site.ShopierStore, shopier.WithLogger(log))
		if err != nil {
			return err
		}
		source = api
	} else {
		fmt.Fprintf(os.Stdout, "Ürünler %s dosyasından okunuyor.\n", *kaynak)
		source = jsonfile.Source{Path: *kaynak, StoreURL: cfg.Site.ShopierStore}
	}
	repo, err := sqlite.Open(filepath.Join(c.data, "catalog.db"), false)
	if err != nil {
		return err
	}
	defer repo.Close()
	images, err := imagestore.New(filepath.Join(c.data, "images"))
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()
	return cli.RunSync(ctx, syncsvc.New(source, repo, images, log), *dry, os.Stdout)
}

func serve(ctx context.Context, log *slog.Logger, args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	var c commonFlags
	c.register(fs)
	addr := fs.String("addr", "", "dinlenecek adres (varsayılan :$PORT veya :8080)")
	fs.Parse(args)
	if *addr == "" {
		*addr = ":8080"
		if p := os.Getenv("PORT"); p != "" {
			*addr = ":" + p
		}
	}

	cfg, err := config.Load(c.config, "")
	if err != nil {
		return err
	}
	repo, err := sqlite.Open(filepath.Join(c.data, "catalog.db"), true)
	if err != nil {
		return err
	}
	defer repo.Close()
	cat, err := catalog.Load(ctx, repo, catalog.Options{
		FeaturedIDs:   cfg.FeaturedIDs,
		NewWindow:     cfg.NewWindow(),
		CategoryOrder: cfg.CategoryOrder,
	})
	if err != nil {
		return err
	}
	srv, err := httpserver.New(httpserver.Options{
		Catalog: cat, Config: cfg, Logger: log, Web: web.FS, ImagesDir: filepath.Join(c.data, "images"),
	})
	if err != nil {
		return err
	}

	hs := &http.Server{
		Addr:              *addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		log.Info("site yayında", "adres", *addr, "ürün", len(cat.All()), "son_senkron", cat.LastSync().Format(time.RFC3339))
		errCh <- hs.ListenAndServe()
	}()
	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	log.Info("kapatılıyor")
	return hs.Shutdown(shutdownCtx)
}
