// Package sqlite ürün anlık görüntüsünü SQLite dosyasında saklar (ports.ProductRepository).
package sqlite

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"liderotomarket/internal/core/domain"
	"liderotomarket/internal/core/ports"
)

//go:embed schema.sql
var schema string

// Repository SQLite deposu.
type Repository struct {
	db *sql.DB
}

var _ ports.ProductRepository = (*Repository)(nil)

// Open veritabanını açar; readOnly=false ise dosya ve şema yoksa oluşturur.
func Open(path string, readOnly bool) (*Repository, error) {
	if readOnly {
		if _, err := os.Stat(path); err != nil {
			return nil, fmt.Errorf("veritabanı bulunamadı (%s). Önce 'make guncelle' çalıştırın: %w", path, err)
		}
	} else if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	dsn := "file:" + path + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	if readOnly {
		dsn += "&mode=ro"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if !readOnly {
		if _, err := db.Exec(schema); err != nil {
			db.Close()
			return nil, fmt.Errorf("şema oluşturulamadı: %w", err)
		}
	}
	return &Repository{db: db}, nil
}

func (r *Repository) Close() error { return r.db.Close() }

const timeLayout = time.RFC3339

func fmtTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(timeLayout)
}

func parseTime(s string) time.Time {
	t, _ := time.Parse(timeLayout, s)
	return t
}

// ReplaceAll tüm ürünleri tek bir işlemde (transaction) yeniler.
func (r *Repository) ReplaceAll(ctx context.Context, products []domain.Product, syncedAt time.Time) (err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()
	for _, q := range []string{"DELETE FROM product_images", "DELETE FROM product_categories", "DELETE FROM products", "DELETE FROM categories"} {
		if _, err = tx.ExecContext(ctx, q); err != nil {
			return err
		}
	}
	insP, err := tx.PrepareContext(ctx, `INSERT INTO products
		(id, title, slug, description_html, description_text, shopier_url, price, old_price, currency, shipping_price,
		 in_stock, stock_quantity, placement_score, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer insP.Close()
	insC, err := tx.PrepareContext(ctx, `INSERT INTO categories (id, title, slug) VALUES (?,?,?)
		ON CONFLICT(id) DO UPDATE SET title=excluded.title, slug=excluded.slug`)
	if err != nil {
		return err
	}
	defer insC.Close()
	insPC, err := tx.PrepareContext(ctx, `INSERT OR IGNORE INTO product_categories (product_id, category_id, position) VALUES (?,?,?)`)
	if err != nil {
		return err
	}
	defer insPC.Close()
	insI, err := tx.PrepareContext(ctx, `INSERT OR REPLACE INTO product_images
		(product_id, placement, source_url, thumb_source_url, path, thumb_path) VALUES (?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer insI.Close()

	for _, p := range products {
		inStock := 0
		if p.InStock {
			inStock = 1
		}
		if _, err = insP.ExecContext(ctx, p.ID, p.Title, p.Slug, p.DescriptionHTML, p.DescriptionText, p.ShopierURL,
			int64(p.Price), int64(p.OldPrice), p.Currency, int64(p.ShippingPrice), inStock, p.StockQuantity,
			p.PlacementScore, fmtTime(p.CreatedAt), fmtTime(p.UpdatedAt)); err != nil {
			return fmt.Errorf("ürün %s: %w", p.ID, err)
		}
		for i, c := range p.Categories {
			id := c.ID
			if id == "" {
				id = "slug:" + c.Slug
			}
			if _, err = insC.ExecContext(ctx, id, c.Title, c.Slug); err != nil {
				return err
			}
			if _, err = insPC.ExecContext(ctx, p.ID, id, i); err != nil {
				return err
			}
		}
		for _, img := range p.Images {
			if _, err = insI.ExecContext(ctx, p.ID, img.Placement, img.SourceURL, img.ThumbSourceURL, img.Path, img.ThumbPath); err != nil {
				return err
			}
		}
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO meta (key, value) VALUES ('last_sync', ?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`, fmtTime(syncedAt)); err != nil {
		return err
	}
	return tx.Commit()
}

// All tüm ürünleri kategori ve görselleriyle döner.
func (r *Repository) All(ctx context.Context) ([]domain.Product, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, title, slug, description_html, description_text, shopier_url, price,
		old_price, currency, shipping_price, in_stock, stock_quantity, placement_score, created_at, updated_at
		FROM products ORDER BY created_at DESC, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var products []domain.Product
	index := map[string]int{}
	for rows.Next() {
		var p domain.Product
		var price, old, ship int64
		var inStock int
		var created, updated string
		if err := rows.Scan(&p.ID, &p.Title, &p.Slug, &p.DescriptionHTML, &p.DescriptionText, &p.ShopierURL, &price,
			&old, &p.Currency, &ship, &inStock, &p.StockQuantity, &p.PlacementScore, &created, &updated); err != nil {
			return nil, err
		}
		p.Price, p.OldPrice, p.ShippingPrice = domain.Money(price), domain.Money(old), domain.Money(ship)
		p.InStock = inStock == 1
		p.CreatedAt, p.UpdatedAt = parseTime(created), parseTime(updated)
		index[p.ID] = len(products)
		products = append(products, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	crows, err := r.db.QueryContext(ctx, `SELECT pc.product_id, c.id, c.title, c.slug FROM product_categories pc
		JOIN categories c ON c.id = pc.category_id ORDER BY pc.product_id, pc.position`)
	if err != nil {
		return nil, err
	}
	defer crows.Close()
	for crows.Next() {
		var pid string
		var c domain.Category
		if err := crows.Scan(&pid, &c.ID, &c.Title, &c.Slug); err != nil {
			return nil, err
		}
		if i, ok := index[pid]; ok {
			products[i].Categories = append(products[i].Categories, c)
		}
	}
	if err := crows.Err(); err != nil {
		return nil, err
	}

	irows, err := r.db.QueryContext(ctx, `SELECT product_id, placement, source_url, thumb_source_url, path, thumb_path
		FROM product_images ORDER BY product_id, placement`)
	if err != nil {
		return nil, err
	}
	defer irows.Close()
	for irows.Next() {
		var pid string
		var img domain.Image
		if err := irows.Scan(&pid, &img.Placement, &img.SourceURL, &img.ThumbSourceURL, &img.Path, &img.ThumbPath); err != nil {
			return nil, err
		}
		if i, ok := index[pid]; ok {
			products[i].Images = append(products[i].Images, img)
		}
	}
	return products, irows.Err()
}

// LastSync son başarılı senkron zamanı (hiç yapılmadıysa sıfır).
func (r *Repository) LastSync(ctx context.Context) (time.Time, error) {
	var v string
	err := r.db.QueryRowContext(ctx, `SELECT value FROM meta WHERE key = 'last_sync'`).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	return parseTime(v), nil
}
