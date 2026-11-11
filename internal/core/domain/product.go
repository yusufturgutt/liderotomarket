package domain

import (
	"math"
	"time"
)

// Category Shopier'deki ürün kategorisi.
type Category struct {
	ID    string
	Title string
	Slug  string
	Count int // katalogda hesaplanır, saklanmaz
}

// Image bir ürün görseli. SourceURL Shopier CDN adresi, Path/ThumbPath ise kendi sunucumuzdaki dosya adları.
type Image struct {
	Placement      int
	SourceURL      string
	ThumbSourceURL string
	Path           string
	ThumbPath      string
}

// Product sitede gösterilen ürün.
type Product struct {
	ID              string
	Title           string
	Slug            string
	DescriptionHTML string // temizlenmiş (sanitize) HTML
	DescriptionText string // düz metin (meta açıklaması, arama)
	ShopierURL      string
	Price           Money // satış fiyatı (indirim varsa indirimli fiyat)
	OldPrice        Money // indirim öncesi fiyat; indirim yoksa 0
	Currency        string
	ShippingPrice   Money
	InStock         bool
	StockQuantity   int
	PlacementScore  int
	Categories      []Category
	Images          []Image
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// HasDiscount eski fiyat satış fiyatından yüksekse true.
func (p Product) HasDiscount() bool {
	return p.OldPrice > p.Price && p.Price > 0
}

// DiscountPercent indirim oranı (yuvarlanmış), indirim yoksa 0.
func (p Product) DiscountPercent() int {
	if !p.HasDiscount() {
		return 0
	}
	return int(math.Round((1 - float64(p.Price)/float64(p.OldPrice)) * 100))
}

// Savings indirim tutarı.
func (p Product) Savings() Money {
	if !p.HasDiscount() {
		return 0
	}
	return p.OldPrice - p.Price
}

// PrimaryCategory ilk kategori; yoksa boş Category.
func (p Product) PrimaryCategory() Category {
	if len(p.Categories) == 0 {
		return Category{}
	}
	return p.Categories[0]
}

// InCategory ürün verilen slug'a sahip kategoride mi.
func (p Product) InCategory(slug string) bool {
	for _, c := range p.Categories {
		if c.Slug == slug {
			return true
		}
	}
	return false
}

// IsNew ürün son window süresi içinde eklendiyse true.
func (p Product) IsNew(now time.Time, window time.Duration) bool {
	if p.CreatedAt.IsZero() || window <= 0 {
		return false
	}
	return now.Sub(p.CreatedAt) <= window
}

// MainImage ilk görsel; yoksa nil.
func (p Product) MainImage() *Image {
	if len(p.Images) == 0 {
		return nil
	}
	return &p.Images[0]
}
