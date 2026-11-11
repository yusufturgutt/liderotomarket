package catalog

import (
	"fmt"
	"testing"
	"time"

	"liderotomarket/internal/core/domain"
	"liderotomarket/internal/core/ports"
)

type fixedClock struct{ t time.Time }

func (f fixedClock) Now() time.Time { return f.t }

var now = time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)

func cat(title string) []domain.Category {
	return []domain.Category{{ID: domain.Slugify(title), Title: title, Slug: domain.Slugify(title)}}
}

func fixture() []domain.Product {
	day := 24 * time.Hour
	return []domain.Product{
		{ID: "1", Title: "Deri Direksiyon Kılıfı", Categories: cat("İç Aksesuar"), Price: 34990, InStock: true, CreatedAt: now.Add(-100 * day)},
		{ID: "2", Title: "3D Havuzlu Paspas Seti", Categories: cat("Oto Paspas"), Price: 89900, OldPrice: 119900, InStock: true, CreatedAt: now.Add(-50 * day)},
		{ID: "3", Title: "Manyetik Telefon Tutucu", Categories: cat("Telefon Aksesuarı"), Price: 24990, InStock: true, CreatedAt: now.Add(-2 * day)},
		{ID: "4", Title: "LED Ayak Altı Aydınlatma", Categories: cat("İç Aksesuar"), Price: 42990, InStock: false, CreatedAt: now.Add(-1 * day)},
		{ID: "5", Title: "Katlanır Bagaj Organizeri", Categories: cat("İç Aksesuar"), Price: 37990, OldPrice: 49990, InStock: true, CreatedAt: now.Add(-20 * day)},
		{ID: "6", Title: "Oto Kokusu Vanilya", Categories: cat("Oto Kokuları"), Price: 20000, InStock: true, CreatedAt: now.Add(-200 * day)},
	}
}

func newCatalog(featured ...string) *Catalog {
	return New(fixture(), Options{
		FeaturedIDs:   featured,
		NewWindow:     30 * 24 * time.Hour,
		CategoryOrder: []string{"Oto Paspas"},
		Clock:         fixedClock{now},
	})
}

func ids(list []domain.Product) []string {
	out := make([]string, len(list))
	for i, p := range list {
		out[i] = p.ID
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestSearchFilters(t *testing.T) {
	c := newCatalog()
	tests := []struct {
		name string
		q    ports.Query
		want []string
	}{
		{"kategori", ports.Query{CategorySlug: "ic-aksesuar", Sort: ports.SortPriceAsc}, []string{"1", "5", "4"}},
		{"indirim", ports.Query{OnlySale: true, Sort: ports.SortDiscount}, []string{"2", "5"}},
		{"yeni", ports.Query{OnlyNew: true, Sort: ports.SortPriceAsc}, []string{"3", "5", "4"}},
		{"arama türkçe", ports.Query{Text: "KILIF"}, []string{"1"}},
		{"arama kategori adı", ports.Query{Text: "kokuları", Sort: ports.SortPriceAsc}, []string{"6"}},
		{"fiyat azalan, tükenen sonda", ports.Query{Sort: ports.SortPriceDesc}, []string{"2", "5", "1", "3", "6", "4"}},
	}
	for _, tt := range tests {
		if got := ids(c.Search(tt.q)); !equal(got, tt.want) {
			t.Errorf("%s: %v, beklenen %v", tt.name, got, tt.want)
		}
	}
}

func TestFeaturedUsesConfigThenFills(t *testing.T) {
	c := newCatalog("6", "4", "2") // 4 tükendi → atlanır
	got := ids(c.Featured(3))
	if len(got) != 3 || got[0] != "6" || got[1] != "2" {
		t.Errorf("Featured = %v", got)
	}
}

func TestCategoriesOrderAndCounts(t *testing.T) {
	c := newCatalog()
	cats := c.Categories()
	if cats[0].Slug != "oto-paspas" {
		t.Errorf("config sırası ilk olmalı: %v", cats[0].Slug)
	}
	if cats[1].Slug != "ic-aksesuar" || cats[1].Count != 3 {
		t.Errorf("ikinci kategori ürün sayısına göre: %+v", cats[1])
	}
	if _, ok := c.CategoryBySlug("oto-kokulari"); !ok {
		t.Error("slug ile kategori bulunmalı")
	}
}

func TestSaleStatsNewestSimilar(t *testing.T) {
	c := newCatalog()
	if s := c.SaleStats(); s.Count != 2 || s.MaxPercent != 25 {
		t.Errorf("SaleStats = %+v", s)
	}
	if got := ids(c.Newest(2)); !equal(got, []string{"3", "5"}) {
		t.Errorf("Newest = %v (tükenen 4 sona gitmeli)", got)
	}
	p, _ := c.ByID("1")
	sim := ids(c.Similar(p, 3))
	if len(sim) != 3 || sim[0] != "5" {
		t.Errorf("Similar = %v", sim)
	}
	for _, id := range sim {
		if id == "1" || id == "4" {
			t.Errorf("Similar kendisini veya tükeneni içermemeli: %v", sim)
		}
	}
	if got := ids(c.ByIDs([]string{"3", "x", "1", "3"})); !equal(got, []string{"3", "1"}) {
		t.Errorf("ByIDs = %v", got)
	}
}

func TestNewFallbackWithoutDates(t *testing.T) {
	var ps []domain.Product
	for i := 1; i <= 15; i++ {
		ps = append(ps, domain.Product{ID: fmt.Sprint(i), Title: "Ü", Price: 100, InStock: true, PlacementScore: i})
	}
	c := New(ps, Options{NewWindow: 30 * 24 * time.Hour, Clock: fixedClock{now}})
	got := c.Search(ports.Query{OnlyNew: true})
	if len(got) != newFallbackCount {
		t.Fatalf("tarih yoksa en yeni %d ürün yeni sayılmalı: %d", newFallbackCount, len(got))
	}
	if p, _ := c.ByID("15"); !c.IsNew(p) {
		t.Error("en yüksek sıradaki ürün yeni olmalı")
	}
	if p, _ := c.ByID("1"); c.IsNew(p) {
		t.Error("en eski ürün yeni olmamalı")
	}
	if ids(c.Newest(1))[0] != "15" {
		t.Errorf("Newest sıralamaya göre: %v", ids(c.Newest(1)))
	}
}
