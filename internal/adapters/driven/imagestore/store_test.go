package imagestore

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveSkipsExistingAndPrunes(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.URL.Path == "/yok.webp" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte("görsel-baytları"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	name, downloaded, err := s.Save(ctx, "51853723-1", srv.URL+"/pictures_large/a.webp")
	if err != nil || !downloaded || filepath.Ext(name) != ".webp" {
		t.Fatalf("ilk indirme: %q %v %v", name, downloaded, err)
	}
	again, downloaded, err := s.Save(ctx, "51853723-1", srv.URL+"/pictures_large/a.webp")
	if err != nil || downloaded || again != name || hits != 1 {
		t.Fatalf("var olan dosya tekrar indirilmemeli: %q %v %v hits=%d", again, downloaded, err, hits)
	}
	noExt, _, err := s.Save(ctx, "2-1", srv.URL+"/gorsel")
	if err != nil || filepath.Ext(noExt) != ".png" {
		t.Fatalf("uzantı Content-Type'tan çıkarılmalı: %q %v", noExt, err)
	}
	if _, _, err := s.Save(ctx, "3-1", srv.URL+"/yok.webp"); err == nil {
		t.Error("404 hata vermeli")
	}
	if _, _, err := s.Save(ctx, "../kaçış", srv.URL+"/a.webp"); err == nil {
		t.Error("yol içeren anahtar reddedilmeli")
	}

	photo := filepath.Join(t.TempDir(), "foto.JPEG")
	os.WriteFile(photo, []byte("yerel"), 0o644)
	local, downloaded, err := s.Save(ctx, "4-1", "file://"+photo)
	if err != nil || !downloaded || filepath.Ext(local) != ".jpg" {
		t.Fatalf("yerel görsel kopyalanmalı: %q %v %v", local, downloaded, err)
	}
	if again, d, _ := s.Save(ctx, "4-1", "file://"+photo); again != local || d {
		t.Error("aynı yerel görsel tekrar kopyalanmamalı")
	}

	removed, err := s.Prune(map[string]bool{name: true})
	if err != nil || removed != 2 {
		t.Fatalf("Prune: %d %v", removed, err)
	}
	if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
		t.Error("tutulan dosya silinmemeli")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("geçici dosya kalmamalı: %d dosya", len(entries))
	}
}
