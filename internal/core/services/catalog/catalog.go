// Package catalog sitenin ürün okuma servisidir. Ürün sayısı küçük olduğu için (~200)
// anlık görüntü açılışta bellekte tutulur ve filtreleme Go içinde yapılır.
package catalog

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"liderotomarket/internal/core/domain"
	"liderotomarket/internal/core/ports"
)

// Options katalog davranışını belirleyen ayarlar (config.yaml'dan gelir).
type Options struct {
	FeaturedIDs   []string
	NewWindow     time.Duration
	CategoryOrder []string // kategori başlıkları
	Clock         ports.Clock
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// Catalog ports.CatalogService uygulaması.
type Catalog struct {
	opts       Options
	products   []domain.Product // önerilen sırada
	byID       map[string]int
	categories []domain.Category
	catBySlug  map[string]int
	featured   map[string]int // id → config sırası
	newByRank  map[string]bool
	lastSync   time.Time
}

// newFallbackCount hiçbir üründe eklenme tarihi yoksa "yeni" sayılacak en yeni ürün sayısı.
const newFallbackCount = 12

var _ ports.CatalogService = (*Catalog)(nil)

// Load ürünleri depodan okuyup kataloğu kurar.
func Load(ctx context.Context, repo ports.ProductRepository, opts Options) (*Catalog, error) {
	products, err := repo.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("ürünler okunamadı: %w", err)
	}
	last, err := repo.LastSync(ctx)
	if err != nil {
		return nil, fmt.Errorf("son senkron zamanı okunamadı: %w", err)
	}
	c := New(products, opts)
	c.lastSync = last
	return c, nil
}

// New verilen ürünlerle katalog oluşturur.
func New(products []domain.Product, opts Options) *Catalog {
	if opts.Clock == nil {
		opts.Clock = systemClock{}
	}
	c := &Catalog{opts: opts, featured: map[string]int{}}
	for i, id := range opts.FeaturedIDs {
		if _, ok := c.featured[id]; !ok {
			c.featured[id] = i
		}
	}
	c.products = append([]domain.Product(nil), products...)
	c.sortRecommended(c.products)
	c.byID = make(map[string]int, len(c.products))
	for i, p := range c.products {
		c.byID[p.ID] = i
	}
	c.buildCategories()
	c.buildNewFallback()
	return c
}

// buildNewFallback: ürünlerin eklenme tarihi bilinmiyorsa (ör. JSON'da createdAt yok),
// Shopier sıralamasına (PlacementScore) göre en yeni ürünler "yeni" sayılır.
func (c *Catalog) buildNewFallback() {
	for _, p := range c.products {
		if !p.CreatedAt.IsZero() {
			return
		}
	}
	list := append([]domain.Product(nil), c.products...)
	sort.SliceStable(list, func(i, j int) bool { return list[i].PlacementScore > list[j].PlacementScore })
	c.newByRank = map[string]bool{}
	for i, p := range list {
		if i >= newFallbackCount {
			break
		}
		c.newByRank[p.ID] = true
	}
}

func (c *Catalog) buildCategories() {
	counts := map[string]*domain.Category{}
	var order []string
	for _, p := range c.products {
		for _, cat := range p.Categories {
			if cat.Slug == "" {
				continue
			}
			if existing, ok := counts[cat.Slug]; ok {
				existing.Count++
				continue
			}
			cc := cat
			cc.Count = 1
			counts[cat.Slug] = &cc
			order = append(order, cat.Slug)
		}
	}
	rank := map[string]int{}
	for i, title := range c.opts.CategoryOrder {
		rank[domain.Slugify(title)] = i
	}
	sort.SliceStable(order, func(i, j int) bool {
		ri, iok := rank[order[i]]
		rj, jok := rank[order[j]]
		switch {
		case iok && jok:
			return ri < rj
		case iok != jok:
			return iok
		}
		ci, cj := counts[order[i]], counts[order[j]]
		if ci.Count != cj.Count {
			return ci.Count > cj.Count
		}
		return domain.Fold(ci.Title) < domain.Fold(cj.Title)
	})
	c.categories = make([]domain.Category, 0, len(order))
	c.catBySlug = make(map[string]int, len(order))
	for i, slug := range order {
		c.categories = append(c.categories, *counts[slug])
		c.catBySlug[slug] = i
	}
}

// sortRecommended: stoktakiler önce, sonra öne çıkanlar (config sırası), Shopier sıralama puanı, en yeni.
func (c *Catalog) sortRecommended(list []domain.Product) {
	sort.SliceStable(list, func(i, j int) bool {
		a, b := list[i], list[j]
		if a.InStock != b.InStock {
			return a.InStock
		}
		fa, aok := c.featured[a.ID]
		fb, bok := c.featured[b.ID]
		if aok != bok {
			return aok
		}
		if aok && bok && fa != fb {
			return fa < fb
		}
		if a.PlacementScore != b.PlacementScore {
			return a.PlacementScore > b.PlacementScore
		}
		if !a.CreatedAt.Equal(b.CreatedAt) {
			return a.CreatedAt.After(b.CreatedAt)
		}
		return a.ID > b.ID
	})
}

func (c *Catalog) All() []domain.Product { return append([]domain.Product(nil), c.products...) }

func (c *Catalog) LastSync() time.Time { return c.lastSync }

func (c *Catalog) IsNew(p domain.Product) bool {
	if c.newByRank != nil {
		return c.newByRank[p.ID]
	}
	return p.IsNew(c.opts.Clock.Now(), c.opts.NewWindow)
}

func (c *Catalog) ByID(id string) (domain.Product, bool) {
	i, ok := c.byID[id]
	if !ok {
		return domain.Product{}, false
	}
	return c.products[i], true
}

// ByIDs verilen sırayı koruyarak bulunan ürünleri döner.
func (c *Catalog) ByIDs(ids []string) []domain.Product {
	out := make([]domain.Product, 0, len(ids))
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		if p, ok := c.ByID(id); ok {
			out = append(out, p)
		}
	}
	return out
}

func (c *Catalog) Categories() []domain.Category {
	return append([]domain.Category(nil), c.categories...)
}

func (c *Catalog) CategoryBySlug(slug string) (domain.Category, bool) {
	i, ok := c.catBySlug[slug]
	if !ok {
		return domain.Category{}, false
	}
	return c.categories[i], true
}

// Search filtreler ve sıralar. Tükenen ürünler her sıralamada sona gider.
func (c *Catalog) Search(q ports.Query) []domain.Product {
	tokens := strings.Fields(domain.Fold(q.Text))
	out := make([]domain.Product, 0, len(c.products))
	for _, p := range c.products {
		if q.CategorySlug != "" && !p.InCategory(q.CategorySlug) {
			continue
		}
		if q.OnlySale && !p.HasDiscount() {
			continue
		}
		if q.OnlyNew && !c.IsNew(p) {
			continue
		}
		if len(tokens) > 0 && !matches(p, tokens) {
			continue
		}
		out = append(out, p)
	}
	less := func(a, b domain.Product) (bool, bool) { return false, false }
	switch q.Sort {
	case ports.SortPriceAsc:
		less = func(a, b domain.Product) (bool, bool) { return a.Price < b.Price, a.Price != b.Price }
	case ports.SortPriceDesc:
		less = func(a, b domain.Product) (bool, bool) { return a.Price > b.Price, a.Price != b.Price }
	case ports.SortDiscount:
		less = func(a, b domain.Product) (bool, bool) {
			pa, pb := a.DiscountPercent(), b.DiscountPercent()
			return pa > pb, pa != pb
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.InStock != b.InStock {
			return a.InStock
		}
		r, decided := less(a, b)
		if decided {
			return r
		}
		return false // önerilen sıra korunur
	})
	return out
}

func matches(p domain.Product, tokens []string) bool {
	hay := domain.Fold(p.Title)
	for _, cat := range p.Categories {
		hay += " " + domain.Fold(cat.Title)
	}
	for _, t := range tokens {
		if !strings.Contains(hay, t) {
			return false
		}
	}
	return true
}

func inStockFirst(list []domain.Product) []domain.Product {
	sort.SliceStable(list, func(i, j int) bool { return list[i].InStock && !list[j].InStock })
	return list
}

func take(list []domain.Product, n int) []domain.Product {
	if n >= 0 && len(list) > n {
		return list[:n]
	}
	return list
}

// Featured config'teki öne çıkan ürünler; eksik kalırsa önerilen sıradaki stoktaki ürünlerle tamamlanır.
func (c *Catalog) Featured(n int) []domain.Product {
	var out []domain.Product
	used := map[string]bool{}
	for _, id := range c.opts.FeaturedIDs {
		if p, ok := c.ByID(id); ok && p.InStock && !used[id] {
			out = append(out, p)
			used[id] = true
		}
	}
	for _, p := range c.products {
		if len(out) >= n {
			break
		}
		if p.InStock && !used[p.ID] {
			out = append(out, p)
			used[p.ID] = true
		}
	}
	return take(out, n)
}

// OnSale indirim oranı en yüksek ürünler.
func (c *Catalog) OnSale(n int) []domain.Product {
	return take(c.Search(ports.Query{OnlySale: true, Sort: ports.SortDiscount}), n)
}

// Newest en son eklenen ürünler (stoktakiler önce).
func (c *Catalog) Newest(n int) []domain.Product {
	list := c.All()
	sort.SliceStable(list, func(i, j int) bool {
		if !list[i].CreatedAt.Equal(list[j].CreatedAt) {
			return list[i].CreatedAt.After(list[j].CreatedAt)
		}
		return list[i].PlacementScore > list[j].PlacementScore
	})
	return take(inStockFirst(list), n)
}

// Similar aynı kategoriden ürünler; yetmezse diğer stoktaki ürünlerle tamamlanır.
func (c *Catalog) Similar(p domain.Product, n int) []domain.Product {
	var out []domain.Product
	used := map[string]bool{p.ID: true}
	if cat := p.PrimaryCategory(); cat.Slug != "" {
		for _, q := range c.Search(ports.Query{CategorySlug: cat.Slug}) {
			if len(out) >= n {
				break
			}
			if q.InStock && !used[q.ID] {
				out = append(out, q)
				used[q.ID] = true
			}
		}
	}
	for _, q := range c.products {
		if len(out) >= n {
			break
		}
		if q.InStock && !used[q.ID] {
			out = append(out, q)
			used[q.ID] = true
		}
	}
	return out
}

func (c *Catalog) SaleStats() ports.SaleStats {
	var s ports.SaleStats
	for _, p := range c.products {
		if p.HasDiscount() {
			s.Count++
			if pct := p.DiscountPercent(); pct > s.MaxPercent {
				s.MaxPercent = pct
			}
		}
	}
	return s
}
