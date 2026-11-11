// Package jsonfile ürünleri elle tutulan bir JSON dosyasından okur (ports.ProductSource).
//
// Shopier ürün API'si kapalı olduğu için ürün listesinin asıl kaynağı urunler.json dosyasıdır.
// Biçim (yalnızca id, name ve price zorunlu):
//
//	[
//	  {
//	    "id": "51853723",                         // Shopier ürün linkinin sonundaki sayı
//	    "name": "Volkswagen Passat Karbon Anahtar Kılıfı",
//	    "category": "Anahtarlık",                 // veya "categories": ["Anahtarlık", "İç Aksesuar"]
//	    "price": "600",                           // TL: 600, "600", "1.199,90" veya 1199.9
//	    "oldPrice": "750",                        // indirim varsa indirim öncesi fiyat
//	    "description": "Karbon görünümlü...",     // düz metin; boş satır yeni paragraf
//	    "images": ["https://cdn.shopier.app/pictures_large/....webp", "gorseller/kendi-fotografim.jpg"],
//	    "inStock": true,                          // yazılmazsa stokta sayılır
//	    "shippingPrice": "100",                   // yazılmazsa "Shopier sipariş sayfasında belirtilir"
//	    "createdAt": "2026-11-01",                // "YENİ" rozeti ve sıralama için
//	    "rank": 1                                 // küçük olan önce (yazılmazsa dosyadaki sıra)
//	  }
//	]
package jsonfile

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"liderotomarket/internal/core/domain"
	"liderotomarket/internal/core/ports"
)

// Source JSON dosyası kaynağı.
type Source struct {
	Path     string
	StoreURL string // https://www.shopier.com/liderotomarket
}

var _ ports.ProductSource = Source{}

// price JSON'da sayı veya metin olabilen TL tutarı.
type price string

func (p *price) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if string(b) == "null" {
		*p = ""
		return nil
	}
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*p = price(s)
		return nil
	}
	*p = price(b)
	return nil
}

// strings1 tek bir metin veya metin listesi.
type strings1 []string

func (s *strings1) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) > 0 && b[0] == '"' {
		var one string
		if err := json.Unmarshal(b, &one); err != nil {
			return err
		}
		*s = strings1{one}
		return nil
	}
	var many []string
	if err := json.Unmarshal(b, &many); err != nil {
		return err
	}
	*s = many
	return nil
}

type item struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Category      strings1 `json:"category"`
	Categories    strings1 `json:"categories"`
	Price         price    `json:"price"`
	OldPrice      price    `json:"oldPrice"`
	Description   string   `json:"description"`
	Images        []string `json:"images"`
	InStock       *bool    `json:"inStock"`
	ShippingPrice price    `json:"shippingPrice"`
	ShopierURL    string   `json:"shopierUrl"`
	CreatedAt     string   `json:"createdAt"`
	Rank          int      `json:"rank"`
}

var idRe = regexp.MustCompile(`^[0-9A-Za-z_-]+$`)

// FetchAll dosyayı okur ve doğrular; hatalı satırları ürün adıyla birlikte bildirir.
func (s Source) FetchAll(context.Context) ([]domain.Product, error) {
	b, err := os.ReadFile(s.Path)
	if err != nil {
		return nil, fmt.Errorf("ürün dosyası okunamadı: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var items []item
	if err := dec.Decode(&items); err != nil {
		return nil, fmt.Errorf("%s hatalı: %w", s.Path, describeJSONError(b, err))
	}

	base := filepath.Dir(s.Path)
	var out []domain.Product
	var problems []string
	seen := map[string]bool{}
	for i, it := range items {
		where := fmt.Sprintf("%d. ürün", i+1)
		if it.Name != "" {
			where += " (" + it.Name + ")"
		}
		p, errs := s.convert(it, base, len(items)-i)
		if seen[p.ID] && p.ID != "" {
			errs = append(errs, "aynı id iki kez kullanılmış: "+p.ID)
		}
		seen[p.ID] = true
		if len(errs) > 0 {
			problems = append(problems, where+": "+strings.Join(errs, "; "))
			continue
		}
		out = append(out, p)
	}
	if len(problems) > 0 {
		return nil, fmt.Errorf("%s içinde %d hata var:\n  - %s", s.Path, len(problems), strings.Join(problems, "\n  - "))
	}
	return out, nil
}

func (s Source) convert(it item, base string, defaultRank int) (domain.Product, []string) {
	var errs []string
	p := domain.Product{
		ID:         strings.TrimSpace(it.ID),
		Title:      strings.TrimSpace(it.Name),
		ShopierURL: strings.TrimSpace(it.ShopierURL),
		Currency:   "TRY",
		InStock:    it.InStock == nil || *it.InStock,
	}
	if p.ID == "" || !idRe.MatchString(p.ID) {
		errs = append(errs, "id boş veya geçersiz")
	}
	if p.Title == "" {
		errs = append(errs, "name boş")
	}
	p.Slug = domain.Slugify(p.Title)
	if p.ShopierURL == "" && s.StoreURL != "" && p.ID != "" {
		p.ShopierURL = strings.TrimRight(s.StoreURL, "/") + "/" + p.ID
	}

	var err error
	if p.Price, err = domain.ParseMoney(string(it.Price)); err != nil || p.Price <= 0 {
		errs = append(errs, fmt.Sprintf("price geçersiz (%q)", string(it.Price)))
	}
	if it.OldPrice != "" {
		if p.OldPrice, err = domain.ParseMoney(string(it.OldPrice)); err != nil {
			errs = append(errs, fmt.Sprintf("oldPrice geçersiz (%q)", string(it.OldPrice)))
		} else if p.OldPrice <= p.Price {
			p.OldPrice = 0 // eski fiyat yeni fiyattan yüksek değilse indirim yok sayılır
		}
	}
	if it.ShippingPrice != "" {
		if p.ShippingPrice, err = domain.ParseMoney(string(it.ShippingPrice)); err != nil {
			errs = append(errs, fmt.Sprintf("shippingPrice geçersiz (%q)", string(it.ShippingPrice)))
		}
	}

	cats := append(append(strings1{}, it.Category...), it.Categories...)
	for _, c := range cats {
		if c = strings.TrimSpace(c); c != "" {
			p.Categories = append(p.Categories, domain.Category{ID: domain.Slugify(c), Title: c, Slug: domain.Slugify(c)})
		}
	}

	p.DescriptionHTML, p.DescriptionText = textToHTML(it.Description)

	for i, img := range it.Images {
		img = strings.TrimSpace(img)
		if img == "" {
			continue
		}
		if !strings.HasPrefix(img, "http://") && !strings.HasPrefix(img, "https://") {
			// Yerel dosya: dosyaya göre göreli yol.
			if !filepath.IsAbs(img) {
				img = filepath.Join(base, img)
			}
			if _, err := os.Stat(img); err != nil {
				errs = append(errs, "görsel bulunamadı: "+img)
				continue
			}
			img = "file://" + img
		}
		im := domain.Image{Placement: i + 1, SourceURL: img}
		if strings.Contains(img, "/pictures_large/") {
			im.ThumbSourceURL = strings.Replace(img, "/pictures_large/", "/pictures_mid/", 1)
		}
		p.Images = append(p.Images, im)
	}

	if it.CreatedAt != "" {
		for _, layout := range []string{"2006-01-02", time.RFC3339, "2006-01-02 15:04", "02.01.2006"} {
			if t, err := time.ParseInLocation(layout, strings.TrimSpace(it.CreatedAt), time.Local); err == nil {
				p.CreatedAt = t.UTC()
				break
			}
		}
		if p.CreatedAt.IsZero() {
			errs = append(errs, fmt.Sprintf("createdAt anlaşılamadı (%q), örnek: 2026-11-01", it.CreatedAt))
		}
	}
	// Katalog yüksek PlacementScore'u önce gösterir; rank'ta küçük olan önce.
	if it.Rank > 0 {
		p.PlacementScore = 1_000_000 - it.Rank
	} else {
		p.PlacementScore = defaultRank
	}
	return p, errs
}

var blankLine = regexp.MustCompile(`\r?\n\s*\r?\n`)

// textToHTML düz metni güvenli paragraflara çevirir.
func textToHTML(s string) (string, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ""
	}
	var b strings.Builder
	for _, para := range blankLine.Split(s, -1) {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		lines := strings.Split(para, "\n")
		for i := range lines {
			lines[i] = html.EscapeString(strings.TrimSpace(lines[i]))
		}
		b.WriteString("<p>" + strings.Join(lines, "<br>") + "</p>")
	}
	return b.String(), strings.Join(strings.Fields(s), " ")
}

// describeJSONError hata konumunu satır numarasıyla verir (elle düzenlemede yardımcı olur).
func describeJSONError(b []byte, err error) error {
	var off int64 = -1
	switch e := err.(type) {
	case *json.SyntaxError:
		off = e.Offset
	case *json.UnmarshalTypeError:
		off = e.Offset
	}
	if off < 0 {
		return err
	}
	line := 1 + bytes.Count(b[:min(int(off), len(b))], []byte("\n"))
	return fmt.Errorf("%d. satır civarı: %w", line, err)
}
