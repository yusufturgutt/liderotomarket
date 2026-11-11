package httpserver

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"liderotomarket/internal/config"
	"liderotomarket/internal/core/domain"
	"liderotomarket/internal/core/services/catalog"
	"liderotomarket/web"
)

type clock struct{ t time.Time }

func (c clock) Now() time.Time { return c.t }

func testServer(t *testing.T, campaign bool) *Server {
	t.Helper()
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	cat := func(title string) []domain.Category {
		return []domain.Category{{ID: domain.Slugify(title), Title: title, Slug: domain.Slugify(title)}}
	}
	products := []domain.Product{
		{ID: "101", Title: "3D Havuzlu Paspas Seti", Slug: "3d-havuzlu-paspas-seti", Categories: cat("Oto Paspas"),
			Price: 89900, OldPrice: 119900, InStock: true, ShopierURL: "https://www.shopier.com/101",
			DescriptionHTML: "<p>Kenarları yükseltilmiş.</p>", DescriptionText: "Kenarları yükseltilmiş.",
			CreatedAt: now.Add(-48 * time.Hour), ShippingPrice: 10000,
			Images: []domain.Image{{Placement: 1, Path: "101-1-a.webp", ThumbPath: "101-1-k.webp"}, {Placement: 2, Path: "101-2-a.webp", ThumbPath: "101-2-a.webp"}}},
		{ID: "102", Title: "Deri Direksiyon Kılıfı", Slug: "deri-direksiyon-kilifi", Categories: cat("İç Aksesuar"),
			Price: 34990, InStock: true, CreatedAt: now.Add(-90 * 24 * time.Hour)},
		{ID: "103", Title: "Vanilya Oto Kokusu", Slug: "vanilya-oto-kokusu", Categories: cat("Oto Kokuları"),
			Price: 20000, InStock: false, CreatedAt: now.Add(-100 * 24 * time.Hour)},
	}
	c := catalog.New(products, catalog.Options{NewWindow: 30 * 24 * time.Hour, Clock: clock{now}})
	cfg := config.Config{
		Site:     config.Site{Name: "Lider Otomarket", BaseURL: "https://liderotomarket.com", ShopierStore: "https://www.shopier.com/liderotomarket", Email: "liderotomarket@gmail.com", Instagram: "liderotomarket", Description: "Otomobil aksesuar ve parçaları."},
		Campaign: config.Banner{Active: campaign, Label: "Kasım İndirimi", Title: "Kasım'a özel indirim", Button: "İndirimli ürünleri gör", Until: "30 Kasım'a kadar"},
		Showcase: config.Banner{Label: "Lider Otomarket", Title: "Aracınız için her şey", Button: "Tüm ürünleri gör"},
		NewDays:  30,
	}
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "101-1-a.webp"), []byte("RIFF....WEBP"), 0o644)
	s, err := New(Options{Catalog: c, Config: cfg, Web: web.FS, ImagesDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return now }
	return s
}

func get(t *testing.T, h http.Handler, path string) (*http.Response, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	res := rec.Result()
	b, _ := io.ReadAll(res.Body)
	return res, string(b)
}

func mustContain(t *testing.T, body string, parts ...string) {
	t.Helper()
	for _, p := range parts {
		if !strings.Contains(body, p) {
			t.Errorf("yanıtta %q bulunamadı", p)
		}
	}
}

func TestPages(t *testing.T) {
	h := testServer(t, true).Handler()

	res, body := get(t, h, "/")
	if res.StatusCode != 200 {
		t.Fatalf("ana sayfa %d", res.StatusCode)
	}
	mustContain(t, body,
		"Kasım&#39;a özel indirim", "1 ürün indirimde", `<span>%</span>25`, "Öne Çıkanlar", "Yeni Ürünler",
		`href="/kategori/oto-paspas"`, `data-fav="101"`, `href="https://www.shopier.com/101"`,
		`"@type":"Store"`, "%25&#39;E VARAN", `/static/css/site.css?v=`)

	res, body = get(t, h, "/urunler?indirim=1")
	if res.StatusCode != 200 {
		t.Fatalf("liste %d", res.StatusCode)
	}
	mustContain(t, body, "<h1>Kasım İndirimi</h1>", "1 ürün", `name="robots" content="noindex`, "İndirimli")
	if strings.Contains(body, "Deri Direksiyon") {
		t.Error("indirim filtresi indirimsiz ürünü göstermemeli")
	}

	_, body = get(t, h, "/kategori/ic-aksesuar")
	mustContain(t, body, "<h1>İç Aksesuar</h1>", `rel="canonical" href="https://liderotomarket.com/kategori/ic-aksesuar"`, "Deri Direksiyon")

	_, body = get(t, h, "/urunler?q=kokusu")
	mustContain(t, body, "Vanilya Oto Kokusu", "Tükendi", `btn-pasif`)

	_, body = get(t, h, "/urunler?q=bulunmayan")
	mustContain(t, body, "Bu filtrelerle eşleşen ürün yok.")

	res, body = get(t, h, "/urun/101-3d-havuzlu-paspas-seti")
	if res.StatusCode != 200 {
		t.Fatalf("ürün %d", res.StatusCode)
	}
	mustContain(t, body, "<h1>3D Havuzlu Paspas Seti</h1>", "899,00 TL", "1.199,00 TL", "300,00 TL tasarruf",
		"<p>Kenarları yükseltilmiş.</p>", "100,00 TL", `"@type":"Product"`, `"price":"899.00"`,
		`og:image" content="https://liderotomarket.com/img/101-1-a.webp"`, `data-galeri-sonraki`, "m-alim")

	res, _ = get(t, h, "/urun/101-yanlis-ad")
	if res.StatusCode != http.StatusMovedPermanently || res.Header.Get("Location") != "/urun/101-3d-havuzlu-paspas-seti" {
		t.Errorf("yanlış slug kalıcı yönlendirilmeli: %d %s", res.StatusCode, res.Header.Get("Location"))
	}
	res, _ = get(t, h, "/urunler?kategori=ic-aksesuar")
	if res.StatusCode != http.StatusMovedPermanently || res.Header.Get("Location") != "/kategori/ic-aksesuar" {
		t.Errorf("tek kategori filtresi kalıcı adrese gitmeli: %d %s", res.StatusCode, res.Header.Get("Location"))
	}

	for _, p := range []string{"/urun/999-yok", "/kategori/yok", "/olmayan-sayfa"} {
		res, body = get(t, h, p)
		if res.StatusCode != 404 {
			t.Errorf("%s 404 olmalı: %d", p, res.StatusCode)
		}
		mustContain(t, body, "Aradığınız sayfa bulunamadı")
	}

	_, body = get(t, h, "/favoriler")
	mustContain(t, body, "data-favori-liste", "Favoriler nasıl çalışır?")

	_, body = get(t, h, "/parca/favoriler?ids=102,yok,101")
	if strings.Index(body, `data-id="102"`) > strings.Index(body, `data-id="101"`) || strings.Contains(body, "<html") {
		t.Errorf("favori parçası sırayı korumalı ve tam sayfa olmamalı: %s", body)
	}

	_, body = get(t, h, "/parca/arama?q=kılıf")
	mustContain(t, body, "Deri Direksiyon Kılıfı", "Tüm sonuçları gör (1)", `href="/urunler?q=k%C4%B1l%C4%B1f"`)
	if strings.Contains(body, "<html") {
		t.Error("arama parçası tam sayfa olmamalı")
	}
	_, body = get(t, h, "/parca/arama?q=yokboyle")
	mustContain(t, body, "ürün bulunamadı")
	if _, body = get(t, h, "/parca/arama?q=a"); strings.TrimSpace(body) != "" {
		t.Errorf("tek harfte sonuç dönmemeli: %q", body)
	}
	_, body = get(t, h, "/")
	mustContain(t, body, `data-arama-ac`, `class="arama-btn"`, `id="m-arama" hidden`)

	_, body = get(t, h, "/sitemap.xml")
	mustContain(t, body, "<loc>https://liderotomarket.com/urun/101-3d-havuzlu-paspas-seti</loc>", "<loc>https://liderotomarket.com/kategori/oto-kokulari</loc>")
	_, body = get(t, h, "/robots.txt")
	mustContain(t, body, "Sitemap: https://liderotomarket.com/sitemap.xml")
}

func TestCampaignOffShowsShowcase(t *testing.T) {
	h := testServer(t, false).Handler()
	_, body := get(t, h, "/")
	mustContain(t, body, "Aracınız için her şey", "3 kategori")
	if strings.Contains(body, "kasim-indirimi") || strings.Contains(body, "menu-link kampanya-link") {
		t.Error("kampanya kapalıyken indirim bölümü ve menü linki gizlenmeli")
	}
}

func TestStaticAndImages(t *testing.T) {
	h := testServer(t, true).Handler()
	req := httptest.NewRequest(http.MethodGet, "/static/css/site.css", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	res := rec.Result()
	if res.StatusCode != 200 || res.Header.Get("Content-Encoding") != "gzip" || !strings.Contains(res.Header.Get("Cache-Control"), "immutable") {
		t.Fatalf("statik dosya: %d %v", res.StatusCode, res.Header)
	}
	zr, err := gzip.NewReader(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(zr)
	if !strings.Contains(string(b), "--mavi: #2757BB") {
		t.Error("CSS içeriği okunamadı")
	}

	res, _ = get(t, h, "/img/101-1-a.webp")
	if res.StatusCode != 200 {
		t.Errorf("görsel %d", res.StatusCode)
	}
	for _, p := range []string{"/img/", "/img/../server.go", "/img/yok.webp"} {
		if res, _ := get(t, h, p); res.StatusCode == 200 {
			t.Errorf("%s erişilememeli", p)
		}
	}
	res, _ = get(t, h, "/favicon.ico")
	if res.StatusCode != 200 || !strings.Contains(res.Header.Get("Content-Type"), "icon") {
		t.Errorf("favicon.ico: %d %s", res.StatusCode, res.Header.Get("Content-Type"))
	}
	res, _ = get(t, h, "/healthz")
	if res.StatusCode != 200 {
		t.Errorf("healthz %d", res.StatusCode)
	}
	if res.Header.Get("Content-Security-Policy") == "" {
		t.Error("güvenlik başlıkları eksik")
	}
}

func TestListStateURL(t *testing.T) {
	tests := []struct {
		st   listState
		want string
	}{
		{listState{}, "/urunler"},
		{listState{Cat: "ic-aksesuar"}, "/kategori/ic-aksesuar"},
		{listState{Cat: "ic-aksesuar", Sale: true}, "/urunler?indirim=1&kategori=ic-aksesuar"},
		{listState{Q: "kılıf", Sort: "artan"}, "/urunler?q=k%C4%B1l%C4%B1f&sirala=artan"},
	}
	for _, tt := range tests {
		if got := tt.st.URL(); got != tt.want {
			t.Errorf("%+v → %q, beklenen %q", tt.st, got, tt.want)
		}
	}
}

func TestCanonicalHostRedirect(t *testing.T) {
	h := testServer(t, true).Handler()
	for _, host := range []string{"www.liderotomarket.com", "liderotomarket.fly.dev"} {
		req := httptest.NewRequest(http.MethodGet, "/urunler?q=koku", nil)
		req.Host = host
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != "https://liderotomarket.com/urunler?q=koku" {
			t.Errorf("%s yönlendirilmeli: %d %s", host, rec.Code, rec.Header().Get("Location"))
		}
	}
	for _, c := range []struct{ host, path string }{{"liderotomarket.com", "/"}, {"localhost:8080", "/"}, {"liderotomarket.fly.dev", "/healthz"}} {
		req := httptest.NewRequest(http.MethodGet, c.path, nil)
		req.Host = c.host
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("%s%s yönlendirilmemeli: %d", c.host, c.path, rec.Code)
		}
	}
}
