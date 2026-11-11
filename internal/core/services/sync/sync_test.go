package sync

import (
	"context"
	"errors"
	"strings"
	gosync "sync"
	"testing"
	"time"

	"liderotomarket/internal/core/domain"
)

type fakeSource struct{ products []domain.Product }

func (f *fakeSource) FetchAll(context.Context) ([]domain.Product, error) {
	// Her çağrıda kopya dön (servis görselleri yerinde değiştirir).
	out := make([]domain.Product, len(f.products))
	for i, p := range f.products {
		p.Images = append([]domain.Image(nil), p.Images...)
		out[i] = p
	}
	return out, nil
}

type fakeRepo struct {
	products []domain.Product
	synced   time.Time
}

func (r *fakeRepo) All(context.Context) ([]domain.Product, error) { return r.products, nil }
func (r *fakeRepo) ReplaceAll(_ context.Context, p []domain.Product, t time.Time) error {
	r.products, r.synced = p, t
	return nil
}
func (r *fakeRepo) LastSync(context.Context) (time.Time, error) { return r.synced, nil }

type fakeImages struct {
	mu   gosync.Mutex
	have map[string]bool
}

func (f *fakeImages) Save(_ context.Context, key, url string) (string, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if strings.Contains(url, "bozuk") {
		return "", false, errors.New("404")
	}
	path := key + ".webp"
	if f.have[path] {
		return path, false, nil
	}
	f.have[path] = true
	return path, true, nil
}

func (f *fakeImages) Prune(keep map[string]bool) (int, error) {
	n := 0
	for p := range f.have {
		if !keep[p] {
			delete(f.have, p)
			n++
		}
	}
	return n, nil
}

func img(n int, url string) domain.Image {
	return domain.Image{Placement: n, SourceURL: url, ThumbSourceURL: url + "?mid"}
}

func TestSync(t *testing.T) {
	ctx := context.Background()
	src := &fakeSource{products: []domain.Product{
		{ID: "1", Title: "A", Price: 100, Images: []domain.Image{img(1, "u1"), img(2, "bozuk")}},
		{ID: "2", Title: "B", Price: 200, Images: []domain.Image{img(1, "u2")}},
	}}
	repo := &fakeRepo{products: []domain.Product{{ID: "1", Title: "A", Price: 100}, {ID: "9", Title: "Silinen"}}}
	store := &fakeImages{have: map[string]bool{"eski.webp": true}}
	svc := New(src, repo, store, nil)

	dry, err := svc.Sync(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if !dry.DryRun || dry.Total != 2 || dry.Added != 1 || dry.Updated != 1 || dry.Removed != 1 || len(repo.products) != 2 || repo.products[1].ID != "9" {
		t.Fatalf("dry-run raporu/yan etkisi yanlış: %+v", dry)
	}

	rep, err := svc.Sync(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if rep.ImagesDownloaded != 2 || rep.ImagesFailed != 1 || rep.ImagesPruned != 1 {
		t.Errorf("görsel raporu yanlış: %+v", rep)
	}
	if len(repo.products) != 2 {
		t.Fatalf("depoda 2 ürün olmalı: %d", len(repo.products))
	}
	p1 := repo.products[0]
	if len(p1.Images) != 1 || p1.Images[0].Path != "1-1.webp" || p1.Images[0].ThumbPath != "1-1-k.webp" {
		t.Errorf("bozuk görsel çıkarılmalı, yollar dolmalı: %+v", p1.Images)
	}

	again, err := svc.Sync(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	// Görseli indirilemeyen ürün 1 "güncellendi" sayılır ve görseli yeniden denenir; diğerleri değişmemeli.
	if again.Added != 0 || again.Updated != 1 || again.Removed != 0 || again.ImagesDownloaded != 0 || again.ImagesPruned != 0 {
		t.Errorf("ikinci senkron yalnızca yeniden denenen ürünü bulmalı: %+v", again)
	}
}

func TestSyncEmptySourceKeepsData(t *testing.T) {
	repo := &fakeRepo{products: []domain.Product{{ID: "1"}}}
	svc := New(&fakeSource{}, repo, &fakeImages{have: map[string]bool{}}, nil)
	if _, err := svc.Sync(context.Background(), false); !errors.Is(err, ErrEmptySource) {
		t.Fatalf("boş kaynak hata vermeli: %v", err)
	}
	if len(repo.products) != 1 {
		t.Error("mevcut veriler silinmemeli")
	}
}
