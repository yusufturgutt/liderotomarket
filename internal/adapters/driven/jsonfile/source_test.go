package jsonfile

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func write(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestFetchAll(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "gorseller"), 0o755)
	write(t, dir, "gorseller/a.jpg", "jpg")
	path := write(t, dir, "urunler.json", `[
 {"id":"51853723","name":"Passat Karbon Anahtar Kılıfı","category":"Anahtarlık","price":"1.199,90","oldPrice":1500,
  "description":"Karbon görünüm.\n\n<b>Dayanıklı</b>","images":["https://cdn.shopier.app/pictures_large/x.webp","gorseller/a.jpg"],
  "createdAt":"2026-11-01","shippingPrice":"100"},
 {"id":"52366573","name":"Vanilya Koku","categories":["Oto Kokuları","İç Aksesuar"],"price":200,"inStock":false,"rank":1}
]`)
	ps, err := Source{Path: path, StoreURL: "https://www.shopier.com/liderotomarket"}.FetchAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 2 {
		t.Fatalf("2 ürün beklendi: %d", len(ps))
	}
	a, b := ps[0], ps[1]
	if a.Price != 119990 || a.OldPrice != 150000 || a.ShippingPrice != 10000 || !a.InStock || a.Slug != "passat-karbon-anahtar-kilifi" {
		t.Errorf("ürün 1: %+v", a)
	}
	if a.ShopierURL != "https://www.shopier.com/liderotomarket/51853723" || a.Categories[0].Slug != "anahtarlik" {
		t.Errorf("link/kategori: %s %+v", a.ShopierURL, a.Categories)
	}
	if a.DescriptionHTML != "<p>Karbon görünüm.</p><p>&lt;b&gt;Dayanıklı&lt;/b&gt;</p>" {
		t.Errorf("açıklama kaçışlanmalı: %q", a.DescriptionHTML)
	}
	if len(a.Images) != 2 || !strings.Contains(a.Images[0].ThumbSourceURL, "pictures_mid") || !strings.HasPrefix(a.Images[1].SourceURL, "file://") {
		t.Errorf("görseller: %+v", a.Images)
	}
	if a.CreatedAt.IsZero() || a.CreatedAt.After(time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("tarih: %v", a.CreatedAt)
	}
	if b.InStock || len(b.Categories) != 2 || b.Price != 20000 || b.PlacementScore <= a.PlacementScore {
		t.Errorf("ürün 2 (rank 1 önce gelmeli): %+v", b)
	}
}

func TestFetchAllReportsProblems(t *testing.T) {
	dir := t.TempDir()
	path := write(t, dir, "urunler.json", `[
 {"id":"1","name":"Fiyatsız"},
 {"id":"2","name":"İyi","price":"10"},
 {"id":"2","name":"Tekrar","price":"10"},
 {"id":"3","name":"Görselsiz","price":"10","images":["yok.jpg"]}
]`)
	_, err := Source{Path: path}.FetchAll(context.Background())
	if err == nil {
		t.Fatal("hata beklendi")
	}
	for _, want := range []string{"Fiyatsız", "price geçersiz", "aynı id", "görsel bulunamadı"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("hata mesajında %q yok: %v", want, err)
		}
	}

	bad := write(t, dir, "bozuk.json", "[\n {\"id\":\"1\",\n  \"name\":\"A\" \"price\":1}\n]")
	_, err = Source{Path: bad}.FetchAll(context.Background())
	if err == nil || !strings.Contains(err.Error(), "3. satır") {
		t.Errorf("sözdizimi hatası satır numarası vermeli: %v", err)
	}
	typo := write(t, dir, "yazim.json", `[{"id":"1","name":"A","pirce":1}]`)
	if _, err = (Source{Path: typo}).FetchAll(context.Background()); err == nil || !strings.Contains(err.Error(), "pirce") {
		t.Errorf("yanlış yazılmış alan adı yakalanmalı: %v", err)
	}
}
