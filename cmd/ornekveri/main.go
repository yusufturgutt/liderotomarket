// ornekveri tasarımdaki örnek ürünlerle yerel bir veritabanı oluşturur.
// Shopier anahtarı olmadan siteyi denemek için:
//
//	go run ./cmd/ornekveri && go run ./cmd/liderotomarket serve -data data-ornek
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"liderotomarket/internal/adapters/driven/sqlite"
	"liderotomarket/internal/core/domain"
)

func main() {
	dir := flag.String("data", "data-ornek", "hedef klasör")
	flag.Parse()

	now := time.Now().UTC()
	day := 24 * time.Hour
	type row struct {
		id, name, cat string
		price, old    domain.Money
		age           int // gün
		stock         bool
	}
	rows := []row{
		{"1", "Deri Direksiyon Kılıfı – Siyah", "İç Aksesuar", 34990, 0, 120, true},
		{"2", "3D Havuzlu Paspas Seti", "Paspas & Bagaj", 89900, 119900, 90, true},
		{"3", "Manyetik Telefon Tutucu – Havalandırma", "Elektronik", 24990, 0, 3, true},
		{"4", "LED Ayak Altı Aydınlatma Kiti", "Aydınlatma", 42990, 0, 5, true},
		{"5", "Katlanır Bagaj Organizeri", "Paspas & Bagaj", 37990, 49990, 60, true},
		{"6", "Hızlı Araç Şarj Cihazı 2×USB-C", "Elektronik", 29990, 0, 8, true},
		{"7", "Seramik Kaplama Sprey 500 ml", "Bakım & Temizlik", 31990, 42990, 70, true},
		{"8", "Mikrofiber Kurulama Havlusu", "Bakım & Temizlik", 14990, 0, 80, true},
		{"9", "Kapı Eşik Koruma Bandı", "Dış Aksesuar", 18990, 0, 12, true},
		{"10", "Cam Rüzgarlığı 4'lü Set", "Dış Aksesuar", 74990, 0, 100, true},
		{"11", "Ortopedik Koltuk Minderi", "İç Aksesuar", 45990, 59990, 110, true},
		{"12", "Araç İçi Çöp Kutusu", "İç Aksesuar", 12990, 0, 130, false},
	}
	var products []domain.Product
	for i, r := range rows {
		p := domain.Product{
			ID: r.id, Title: r.name, Slug: domain.Slugify(r.name),
			DescriptionHTML: "<p>Örnek ürün açıklaması. Gerçek açıklama Shopier'den gelecek.</p>",
			DescriptionText: "Örnek ürün açıklaması. Gerçek açıklama Shopier'den gelecek.",
			ShopierURL:      "https://www.shopier.com/liderotomarket",
			Price:           r.price, Currency: "TRY", InStock: r.stock, StockQuantity: 5,
			PlacementScore: len(rows) - i,
			Categories:     []domain.Category{{ID: domain.Slugify(r.cat), Title: r.cat, Slug: domain.Slugify(r.cat)}},
			CreatedAt:      now.Add(-time.Duration(r.age) * day),
			UpdatedAt:      now,
		}
		if r.old > 0 {
			p.OldPrice = r.old
		}
		products = append(products, p)
	}

	os.RemoveAll(*dir)
	repo, err := sqlite.Open(filepath.Join(*dir, "catalog.db"), false)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer repo.Close()
	if err := os.MkdirAll(filepath.Join(*dir, "images"), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := repo.ReplaceAll(context.Background(), products, now); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("%d örnek ürün %s klasörüne yazıldı.\n", len(products), *dir)
}
