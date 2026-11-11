package shopier

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const page1 = `[
 {"id":"52366573","title":"Kiss Vanilla Mystery Oto Araç Kokusu","description":"Uzun süre kalıcı.\n\nAraç içi için.","url":"https://www.shopier.com/52366573",
  "dateCreated":"2026-11-01T10:00:00+03:00","dateUpdated":"2026-11-02T10:00:00+03:00","stockStatus":"outOfStock","stockQuantity":0,
  "priceData":{"currency":"TRY","price":"200.00","discountedPrice":"","discount":false,"shippingPrice":"100.00"},
  "media":[{"id":"a","type":"image","url":"https://cdn.shopier.app/pictures_large/liderotomarket_2.webp","placement":2},
           {"id":"b","type":"image","url":"https://cdn.shopier.app/pictures_large/liderotomarket_1.webp","placement":1}],
  "categories":[{"id":"7","title":"Oto Kokuları"}]},
 {"id":51853723,"title":"Volkswagen Passat Karbon Anahtar Kılıfı","description":"<p>Karbon <b>görünüm</b><script>alert(1)</script></p>",
  "dateCreated":"2026-10-01 09:00:00","stockStatus":"inStock","stockQuantity":"5","placementScore":12.0,
  "priceData":{"currency":"TRY","price":"799.90","discountedPrice":"600.00","discount":true,"shippingPrice":"0"},
  "media":[],"categories":[{"id":3,"title":"Anahtarlık"}],
  "variants":[{"stockStatus":"outOfStock","stockQuantity":0},{"stockStatus":"inStock","stockQuantity":2}]}
]`

func TestFetchAllPaginatesAndMaps(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if got := r.Header.Get("Authorization"); got != "Bearer test-pat" {
			t.Errorf("Authorization başlığı yanlış: %q", got)
		}
		if r.URL.Path != "/products" || r.URL.Query().Get("limit") != "50" {
			t.Errorf("beklenmeyen istek: %s", r.URL)
		}
		w.Header().Set("Shopier-Pagination-Total-Pages", "2")
		switch r.URL.Query().Get("page") {
		case "1":
			fmt.Fprint(w, page1)
		case "2":
			fmt.Fprint(w, `[{"id":"3","title":"Üçüncü","priceData":{"price":"10"},"stockStatus":"inStock"}]`)
		default:
			t.Errorf("fazladan sayfa istendi: %s", r.URL)
		}
	}))
	defer srv.Close()

	c, err := New("test-pat", "https://www.shopier.com/liderotomarket", WithBaseURL(srv.URL), WithPageDelay(0))
	if err != nil {
		t.Fatal(err)
	}
	products, err := c.FetchAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(products) != 3 {
		t.Fatalf("calls=%d ürün=%d", calls, len(products))
	}

	a := products[0]
	if a.ID != "52366573" || a.InStock || a.Price != 20000 || a.OldPrice != 0 || a.ShippingPrice != 10000 {
		t.Errorf("ürün 1 yanlış eşlendi: %+v", a)
	}
	if len(a.Images) != 2 || a.Images[0].Placement != 1 || !strings.Contains(a.Images[0].ThumbSourceURL, "/pictures_mid/") {
		t.Errorf("görseller sıralanmalı ve küçük görsel türetilmeli: %+v", a.Images)
	}
	if a.DescriptionHTML != "<p>Uzun süre kalıcı.</p><p>Araç içi için.</p>" || a.DescriptionText != "Uzun süre kalıcı. Araç içi için." {
		t.Errorf("düz metin açıklama: %q / %q", a.DescriptionHTML, a.DescriptionText)
	}
	if a.Categories[0].Slug != "oto-kokulari" || a.CreatedAt.IsZero() {
		t.Errorf("kategori/tarih: %+v %v", a.Categories, a.CreatedAt)
	}

	b := products[1]
	if b.ID != "51853723" || !b.InStock || b.Price != 60000 || b.OldPrice != 79990 || b.DiscountPercent() != 25 || b.StockQuantity != 5 || b.PlacementScore != 12 {
		t.Errorf("ürün 2 yanlış eşlendi: %+v", b)
	}
	if strings.Contains(b.DescriptionHTML, "script") || !strings.Contains(b.DescriptionHTML, "<b>görünüm</b>") {
		t.Errorf("HTML temizlenmeli: %q", b.DescriptionHTML)
	}
	if b.CreatedAt != time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC) {
		t.Errorf("tarih: %v", b.CreatedAt)
	}
	if products[2].ShopierURL != "https://www.shopier.com/liderotomarket/3" {
		t.Errorf("link yoksa mağaza linkinden üretilmeli: %q", products[2].ShopierURL)
	}
}

func TestFetchAllUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	c, _ := New("kötü", "", WithBaseURL(srv.URL))
	_, err := c.FetchAll(context.Background())
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("401 hatası anlaşılır olmalı: %v", err)
	}
}

func TestFetchAllRetriesServerErrors(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		fmt.Fprint(w, `[{"id":"1","title":"A","priceData":{"price":"1"}}]`)
	}))
	defer srv.Close()
	c, _ := New("pat", "", WithBaseURL(srv.URL))
	products, err := c.FetchAll(context.Background())
	if err != nil || len(products) != 1 || calls != 2 {
		t.Fatalf("yeniden denemeli: err=%v ürün=%d calls=%d", err, len(products), calls)
	}
}

func TestNewRequiresToken(t *testing.T) {
	if _, err := New("", ""); err == nil {
		t.Error("anahtar yoksa hata vermeli")
	}
}
