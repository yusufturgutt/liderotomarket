package shopier

import (
	"bytes"
	"encoding/json"
	"html"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/microcosm-cc/bluemonday"

	"liderotomarket/internal/core/domain"
)

// flexString JSON'da string, sayı veya null olabilen alanlar için.
type flexString string

func (f *flexString) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		*f = ""
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*f = flexString(s)
		return nil
	}
	*f = flexString(string(b))
	return nil
}

// flexInt JSON'da sayı veya string olabilen tamsayılar için.
type flexInt int

func (f *flexInt) UnmarshalJSON(b []byte) error {
	var s flexString
	if err := s.UnmarshalJSON(b); err != nil {
		return err
	}
	if s == "" {
		*f = 0
		return nil
	}
	var n json.Number = json.Number(s)
	if i, err := n.Int64(); err == nil {
		*f = flexInt(i)
		return nil
	}
	if fl, err := n.Float64(); err == nil {
		*f = flexInt(int(fl))
		return nil
	}
	*f = 0
	return nil
}

// apiProduct Shopier ürün modeli (https://developer.shopier.com/reference/the-product-model).
type apiProduct struct {
	ID             flexString `json:"id"`
	Title          string     `json:"title"`
	Description    string     `json:"description"`
	URL            string     `json:"url"`
	DateCreated    string     `json:"dateCreated"`
	DateUpdated    string     `json:"dateUpdated"`
	StockStatus    string     `json:"stockStatus"`
	StockQuantity  flexInt    `json:"stockQuantity"`
	PlacementScore flexInt    `json:"placementScore"`
	PriceData      struct {
		Currency        string     `json:"currency"`
		Price           flexString `json:"price"`
		DiscountedPrice flexString `json:"discountedPrice"`
		Discount        bool       `json:"discount"`
		ShippingPrice   flexString `json:"shippingPrice"`
	} `json:"priceData"`
	Media []struct {
		Type      string  `json:"type"`
		URL       string  `json:"url"`
		Placement flexInt `json:"placement"`
	} `json:"media"`
	Categories []struct {
		ID    flexString `json:"id"`
		Title string     `json:"title"`
	} `json:"categories"`
	Variants []struct {
		StockStatus   string  `json:"stockStatus"`
		StockQuantity flexInt `json:"stockQuantity"`
	} `json:"variants"`
}

var (
	descPolicy = func() *bluemonday.Policy {
		p := bluemonday.NewPolicy()
		p.AllowElements("p", "br", "b", "strong", "i", "em", "u", "ul", "ol", "li", "h3", "h4", "span", "div")
		return p
	}()
	stripPolicy = bluemonday.StrictPolicy()
	spaceRe     = regexp.MustCompile(`\s+`)
	pictureRe   = regexp.MustCompile(`/pictures_(large|original|big)/`)
	tagRe       = regexp.MustCompile(`<[a-zA-Z/][^>]*>`)
)

func mapProduct(a apiProduct, storeURL string) domain.Product {
	p := domain.Product{
		ID:             strings.TrimSpace(string(a.ID)),
		Title:          strings.TrimSpace(a.Title),
		ShopierURL:     strings.TrimSpace(a.URL),
		Currency:       a.PriceData.Currency,
		StockQuantity:  int(a.StockQuantity),
		PlacementScore: int(a.PlacementScore),
		CreatedAt:      parseTime(a.DateCreated),
		UpdatedAt:      parseTime(a.DateUpdated),
	}
	p.Slug = domain.Slugify(p.Title)
	if p.ShopierURL == "" && storeURL != "" && p.ID != "" {
		p.ShopierURL = strings.TrimRight(storeURL, "/") + "/" + p.ID
	}

	price, _ := domain.ParseMoney(string(a.PriceData.Price))
	discounted, _ := domain.ParseMoney(string(a.PriceData.DiscountedPrice))
	p.Price = price
	if a.PriceData.Discount && discounted > 0 && discounted < price {
		p.Price = discounted
		p.OldPrice = price
	}
	p.ShippingPrice, _ = domain.ParseMoney(string(a.PriceData.ShippingPrice))

	// Stok: ürün veya varyantlardan biri stoktaysa satılabilir.
	p.InStock = a.StockStatus != "outOfStock"
	for _, v := range a.Variants {
		if v.StockStatus == "inStock" {
			p.InStock = true
		}
	}

	p.DescriptionHTML, p.DescriptionText = cleanDescription(a.Description)

	for _, c := range a.Categories {
		title := strings.TrimSpace(c.Title)
		if title == "" {
			continue
		}
		p.Categories = append(p.Categories, domain.Category{ID: string(c.ID), Title: title, Slug: domain.Slugify(title)})
	}

	for _, m := range a.Media {
		if m.URL == "" || (m.Type != "" && m.Type != "image") {
			continue
		}
		img := domain.Image{Placement: int(m.Placement), SourceURL: m.URL}
		if pictureRe.MatchString(m.URL) {
			img.ThumbSourceURL = pictureRe.ReplaceAllString(m.URL, "/pictures_mid/")
		}
		p.Images = append(p.Images, img)
	}
	sort.SliceStable(p.Images, func(i, j int) bool { return p.Images[i].Placement < p.Images[j].Placement })
	for i := range p.Images {
		if p.Images[i].Placement <= 0 {
			p.Images[i].Placement = i + 1
		}
	}
	return p
}

// cleanDescription Shopier açıklamasını güvenli HTML'e ve düz metne çevirir.
func cleanDescription(raw string) (htmlOut, text string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ""
	}
	if !tagRe.MatchString(raw) {
		// Düz metin: paragraflara böl, satır sonlarını koru.
		var b strings.Builder
		for _, para := range regexp.MustCompile(`\r?\n\s*\r?\n`).Split(raw, -1) {
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
		htmlOut = b.String()
	} else {
		htmlOut = strings.TrimSpace(descPolicy.Sanitize(raw))
	}
	text = html.UnescapeString(stripPolicy.Sanitize(strings.NewReplacer("<br>", " ", "<br/>", " ", "</p>", " ").Replace(raw)))
	text = strings.TrimSpace(spaceRe.ReplaceAllString(text, " "))
	return htmlOut, text
}

func parseTime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}
