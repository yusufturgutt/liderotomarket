// Package ports hexagonal mimarinin sınırlarını tanımlar.
// Core (domain + services) yalnızca bu arayüzlere bağımlıdır; somut uygulamalar adapters altındadır.
package ports

import (
	"context"
	"time"

	"liderotomarket/internal/core/domain"
)

// ---- Driven (çıkış) portları: core'un dış dünyadan istedikleri ----

// ProductSource ürünlerin geldiği kaynak (Shopier API).
// Dönen ürünlerde Images[].SourceURL doludur, Path alanları boştur.
type ProductSource interface {
	FetchAll(ctx context.Context) ([]domain.Product, error)
}

// ProductRepository ürün anlık görüntüsünün (snapshot) saklandığı yer (SQLite).
type ProductRepository interface {
	All(ctx context.Context) ([]domain.Product, error)
	ReplaceAll(ctx context.Context, products []domain.Product, syncedAt time.Time) error
	LastSync(ctx context.Context) (time.Time, error)
}

// ImageStore görselleri indirip yerelde saklar.
type ImageStore interface {
	// Save görseli indirir (zaten varsa atlar) ve sunucudaki dosya adını döner.
	Save(ctx context.Context, key, sourceURL string) (path string, downloaded bool, err error)
	// Prune keep içinde olmayan dosyaları siler.
	Prune(keep map[string]bool) (removed int, err error)
}

// Clock test edilebilirlik için zaman kaynağı.
type Clock interface {
	Now() time.Time
}

// ---- Driving (giriş) portları: dış dünyanın core'dan istedikleri ----

// SyncReport senkronizasyon özeti.
type SyncReport struct {
	Total            int
	Added            int
	Updated          int
	Removed          int
	ImagesTotal      int
	ImagesDownloaded int
	ImagesFailed     int
	ImagesPruned     int
	DryRun           bool
	Duration         time.Duration
}

// SyncService Shopier → yerel veri senkronizasyonu.
type SyncService interface {
	Sync(ctx context.Context, dryRun bool) (SyncReport, error)
}

// Sort sıralama seçenekleri.
type Sort string

const (
	SortRecommended Sort = "onerilen"
	SortPriceAsc    Sort = "artan"
	SortPriceDesc   Sort = "azalan"
	SortDiscount    Sort = "indirim"
)

// Query ürün listeleme filtresi.
type Query struct {
	CategorySlug string
	OnlySale     bool
	OnlyNew      bool
	Text         string
	Sort         Sort
}

// SaleStats kampanya bandı için indirim özeti.
type SaleStats struct {
	Count      int
	MaxPercent int
}

// CatalogService sitenin ürün okuma ihtiyaçları.
type CatalogService interface {
	All() []domain.Product
	Search(q Query) []domain.Product
	ByID(id string) (domain.Product, bool)
	ByIDs(ids []string) []domain.Product
	Categories() []domain.Category
	CategoryBySlug(slug string) (domain.Category, bool)
	Featured(n int) []domain.Product
	OnSale(n int) []domain.Product
	Newest(n int) []domain.Product
	Similar(p domain.Product, n int) []domain.Product
	SaleStats() SaleStats
	IsNew(p domain.Product) bool
	LastSync() time.Time
}
