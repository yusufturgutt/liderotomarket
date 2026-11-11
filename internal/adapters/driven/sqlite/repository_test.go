package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"liderotomarket/internal/core/domain"
)

func TestReplaceAllAndRead(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "catalog.db")
	repo, err := Open(path, false)
	if err != nil {
		t.Fatal(err)
	}
	created := time.Date(2026, 11, 1, 7, 0, 0, 0, time.UTC)
	cat := domain.Category{ID: "7", Title: "Oto Kokuları", Slug: "oto-kokulari"}
	in := []domain.Product{
		{ID: "1", Title: "Koku", Slug: "koku", Price: 20000, OldPrice: 25000, InStock: true, CreatedAt: created,
			Categories: []domain.Category{cat},
			Images:     []domain.Image{{Placement: 2, SourceURL: "s2", Path: "1-2.webp", ThumbPath: "1-2-k.webp"}, {Placement: 1, SourceURL: "s1", Path: "1-1.webp", ThumbPath: "1-1.webp"}}},
		{ID: "2", Title: "Anahtarlık", Slug: "anahtarlik", Price: 50000, InStock: false, Categories: []domain.Category{cat}},
	}
	synced := time.Date(2026, 11, 11, 9, 30, 0, 0, time.UTC)
	if err := repo.ReplaceAll(ctx, in, synced); err != nil {
		t.Fatal(err)
	}
	// İkinci yazım ilkini tamamen değiştirmeli.
	if err := repo.ReplaceAll(ctx, in[:1], synced); err != nil {
		t.Fatal(err)
	}
	repo.Close()

	ro, err := Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	out, err := ro.All(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("1 ürün beklendi, %d", len(out))
	}
	p := out[0]
	if p.Price != 20000 || p.OldPrice != 25000 || !p.InStock || !p.CreatedAt.Equal(created) {
		t.Errorf("alanlar korunmalı: %+v", p)
	}
	if len(p.Categories) != 1 || p.Categories[0].Title != "Oto Kokuları" {
		t.Errorf("kategoriler: %+v", p.Categories)
	}
	if len(p.Images) != 2 || p.Images[0].Placement != 1 || p.Images[1].ThumbPath != "1-2-k.webp" {
		t.Errorf("görseller sıralı gelmeli: %+v", p.Images)
	}
	last, err := ro.LastSync(ctx)
	if err != nil || !last.Equal(synced) {
		t.Errorf("LastSync = %v, %v", last, err)
	}
	if err := ro.ReplaceAll(ctx, in, synced); err == nil {
		t.Error("salt okunur bağlantı yazamamalı")
	}
}

func TestOpenReadOnlyMissing(t *testing.T) {
	if _, err := Open(filepath.Join(t.TempDir(), "yok.db"), true); err == nil {
		t.Error("olmayan dosya salt okunur açılamamalı")
	}
}
