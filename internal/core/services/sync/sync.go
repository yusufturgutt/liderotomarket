// Package sync Shopier'deki ürünleri yerel depoya (SQLite + görseller) aktarır.
package sync

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	gosync "sync"
	"time"

	"liderotomarket/internal/core/domain"
	"liderotomarket/internal/core/ports"
)

// Service ports.SyncService uygulaması.
type Service struct {
	source  ports.ProductSource
	repo    ports.ProductRepository
	images  ports.ImageStore
	clock   ports.Clock
	log     *slog.Logger
	workers int
}

var _ ports.SyncService = (*Service)(nil)

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// New senkron servisini oluşturur.
func New(source ports.ProductSource, repo ports.ProductRepository, images ports.ImageStore, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{source: source, repo: repo, images: images, clock: systemClock{}, log: log, workers: 8}
}

// WithClock testler için saat değiştirir.
func (s *Service) WithClock(c ports.Clock) *Service { s.clock = c; return s }

// ErrEmptySource kaynak hiç ürün dönmediğinde verilir; mevcut site yanlışlıkla boşaltılmasın diye.
var ErrEmptySource = errors.New("Shopier hiç ürün döndürmedi; mevcut veriler korunuyor")

func (s *Service) Sync(ctx context.Context, dryRun bool) (ports.SyncReport, error) {
	start := s.clock.Now()
	rep := ports.SyncReport{DryRun: dryRun}

	existing, err := s.repo.All(ctx)
	if err != nil {
		return rep, fmt.Errorf("mevcut ürünler okunamadı: %w", err)
	}
	fetched, err := s.source.FetchAll(ctx)
	if err != nil {
		return rep, fmt.Errorf("Shopier'den ürünler alınamadı: %w", err)
	}
	if len(fetched) == 0 {
		return rep, ErrEmptySource
	}

	old := make(map[string]domain.Product, len(existing))
	for _, p := range existing {
		old[p.ID] = p
	}
	seen := make(map[string]bool, len(fetched))
	for _, p := range fetched {
		seen[p.ID] = true
		prev, ok := old[p.ID]
		switch {
		case !ok:
			rep.Added++
		case changed(prev, p):
			rep.Updated++
		}
		rep.ImagesTotal += len(p.Images)
	}
	for id := range old {
		if !seen[id] {
			rep.Removed++
		}
	}
	rep.Total = len(fetched)

	if dryRun {
		rep.Duration = s.clock.Now().Sub(start)
		return rep, nil
	}

	keep := s.downloadImages(ctx, fetched, &rep)
	if err := ctx.Err(); err != nil {
		return rep, err
	}

	if err := s.repo.ReplaceAll(ctx, fetched, s.clock.Now()); err != nil {
		return rep, fmt.Errorf("ürünler kaydedilemedi: %w", err)
	}
	pruned, err := s.images.Prune(keep)
	if err != nil {
		s.log.Warn("eski görseller silinemedi", "hata", err)
	}
	rep.ImagesPruned = pruned
	rep.Duration = s.clock.Now().Sub(start)
	return rep, nil
}

type imageJob struct {
	product, image int
	thumb          bool
	key, url       string
}

// downloadImages görselleri paralel indirir, ürünlerin Path alanlarını doldurur, başarısız görselleri çıkarır.
func (s *Service) downloadImages(ctx context.Context, products []domain.Product, rep *ports.SyncReport) map[string]bool {
	var jobs []imageJob
	for pi := range products {
		for ii, img := range products[pi].Images {
			key := fmt.Sprintf("%s-%d", products[pi].ID, img.Placement)
			jobs = append(jobs, imageJob{product: pi, image: ii, key: key, url: img.SourceURL})
			if img.ThumbSourceURL != "" && img.ThumbSourceURL != img.SourceURL {
				jobs = append(jobs, imageJob{product: pi, image: ii, thumb: true, key: key + "-k", url: img.ThumbSourceURL})
			}
		}
	}

	var mu gosync.Mutex
	keep := map[string]bool{}
	ch := make(chan imageJob)
	var wg gosync.WaitGroup
	for w := 0; w < s.workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range ch {
				path, downloaded, err := s.images.Save(ctx, j.key, j.url)
				mu.Lock()
				if err != nil {
					if !j.thumb {
						rep.ImagesFailed++
					}
					s.log.Warn("görsel indirilemedi", "ürün", products[j.product].ID, "url", j.url, "hata", err)
				} else {
					keep[path] = true
					img := &products[j.product].Images[j.image]
					if j.thumb {
						img.ThumbPath = path
					} else {
						img.Path = path
						if downloaded {
							rep.ImagesDownloaded++
						}
					}
				}
				mu.Unlock()
			}
		}()
	}
	for _, j := range jobs {
		select {
		case ch <- j:
		case <-ctx.Done():
		}
		if ctx.Err() != nil {
			break
		}
	}
	close(ch)
	wg.Wait()

	for pi := range products {
		imgs := products[pi].Images[:0]
		for _, img := range products[pi].Images {
			if img.Path == "" {
				continue
			}
			if img.ThumbPath == "" {
				img.ThumbPath = img.Path
			}
			imgs = append(imgs, img)
		}
		products[pi].Images = imgs
	}
	return keep
}

func changed(a, b domain.Product) bool {
	if !a.UpdatedAt.Equal(b.UpdatedAt) {
		return true
	}
	if a.Title != b.Title || a.Price != b.Price || a.OldPrice != b.OldPrice || a.InStock != b.InStock ||
		a.DescriptionHTML != b.DescriptionHTML || len(a.Images) != len(b.Images) || len(a.Categories) != len(b.Categories) {
		return true
	}
	for i := range a.Images {
		if a.Images[i].SourceURL != b.Images[i].SourceURL {
			return true
		}
	}
	for i := range a.Categories {
		if a.Categories[i].ID != b.Categories[i].ID {
			return true
		}
	}
	return false
}
