// Package httpserver sitenin HTTP arayüzüdür (driving adapter): sayfaları sunucu tarafında
// html/template ile üretir, statik dosyaları ve ürün görsellerini sunar.
package httpserver

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"time"

	"liderotomarket/internal/config"
	"liderotomarket/internal/core/ports"
)

// Server HTTP sunucusu.
type Server struct {
	catalog   ports.CatalogService
	cfg       config.Config
	log       *slog.Logger
	static    fs.FS
	imagesDir string
	pages     map[string]*template.Template
	assetVer  map[string]string
	now       func() time.Time
}

// Options sunucu bağımlılıkları.
type Options struct {
	Catalog   ports.CatalogService
	Config    config.Config
	Logger    *slog.Logger
	Web       fs.FS // templates/ ve static/ içeren dosya sistemi
	ImagesDir string
}

var pageFiles = []string{"anasayfa", "urunler", "urun", "favoriler", "bulunamadi", "parca-favoriler", "parca-arama"}

// New şablonları derler ve sunucuyu hazırlar.
func New(o Options) (*Server, error) {
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	static, err := fs.Sub(o.Web, "static")
	if err != nil {
		return nil, err
	}
	s := &Server{
		catalog:   o.Catalog,
		cfg:       o.Config,
		log:       o.Logger,
		static:    static,
		imagesDir: o.ImagesDir,
		pages:     map[string]*template.Template{},
		assetVer:  map[string]string{},
		now:       time.Now,
	}
	if err := fs.WalkDir(static, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(static, p)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		s.assetVer[p] = hex.EncodeToString(sum[:])[:10]
		return nil
	}); err != nil {
		return nil, err
	}

	funcs := template.FuncMap{
		"ikon":    icon,
		"asset":   s.assetURL,
		"add1":    func(i int) int { return i + 1 },
		"karusel": carousel,
	}
	for _, name := range pageFiles {
		t, err := template.New(name).Funcs(funcs).ParseFS(o.Web,
			"templates/layout.html", "templates/parcalar.html", "templates/"+name+".html")
		if err != nil {
			return nil, fmt.Errorf("şablon %s: %w", name, err)
		}
		s.pages[name] = t
	}
	return s, nil
}

func (s *Server) assetURL(p string) string {
	if v, ok := s.assetVer[p]; ok {
		return "/static/" + p + "?v=" + v
	}
	return "/static/" + p
}

// Handler tüm rotaları ve ara katmanları döner.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleHome)
	mux.HandleFunc("GET /urunler", s.handleList)
	mux.HandleFunc("GET /kategori/{slug}", s.handleList)
	mux.HandleFunc("GET /urun/{key}", s.handleProduct)
	mux.HandleFunc("GET /favoriler", s.handleFavorites)
	mux.HandleFunc("GET /parca/favoriler", s.handleFavoritesFragment)
	mux.HandleFunc("GET /parca/arama", s.handleSearchFragment)
	mux.HandleFunc("GET /sitemap.xml", s.handleSitemap)
	mux.HandleFunc("GET /robots.txt", s.handleRobots)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		fmt.Fprint(w, "ok")
	})
	mux.Handle("GET /static/", http.StripPrefix("/static/", immutable(http.FileServerFS(s.static))))
	mux.Handle("GET /img/", http.StripPrefix("/img/", immutable(noDirListing(http.FileServer(http.Dir(s.imagesDir))))))
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=86400")
		http.ServeFileFS(w, r, s.static, "img/favicon.ico")
	})
	mux.HandleFunc("/", s.handleNotFound)
	return recoverer(s.log, securityHeaders(gzipHandler(mux)))
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, status int, page string, data layoutVM) {
	t, ok := s.pages[page]
	if !ok {
		http.Error(w, "şablon yok", http.StatusInternalServerError)
		return
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout", data); err != nil {
		s.log.Error("şablon hatası", "sayfa", page, "hata", err)
		http.Error(w, "Sayfa oluşturulamadı", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if status == http.StatusOK {
		w.Header().Set("Cache-Control", "public, max-age=300")
	} else {
		w.Header().Set("Cache-Control", "no-store")
	}
	w.WriteHeader(status)
	buf.WriteTo(w)
}

func immutable(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		h.ServeHTTP(w, r)
	})
}

func noDirListing(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "" || strings.HasSuffix(r.URL.Path, "/") || strings.Contains(path.Base(r.URL.Path), "..") {
			http.NotFound(w, r)
			return
		}
		h.ServeHTTP(w, r)
	})
}
