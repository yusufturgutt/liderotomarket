package httpserver

import (
	"encoding/json"
	"html/template"
	"net/url"
	"strconv"
	"strings"

	"liderotomarket/internal/config"
	"liderotomarket/internal/core/domain"
	"liderotomarket/internal/core/ports"
)

// ---- Ortak görünüm modelleri ----

type cardVM struct {
	ID, Name, URL, ShopierURL, Category string
	Price, Old                          string
	HasOld, IsNew, InStock              bool
	Pct                                 int
	Img, Big                            string
}

type catVM struct {
	Name, Slug, URL string
	Count           int
	Current         bool
}

type layoutVM struct {
	Site         config.Site
	InstagramURL string
	Year         int

	Title, Description, Canonical, OGImage, OGType string
	JSONLD                                         template.JS
	NoIndex                                        bool

	Nav         string // aktif menü: urunler | yeni | indirim | favoriler
	Home        bool
	SearchQuery string

	Cats          []catVM
	CampaignOn    bool
	CampaignLabel string
	Sale          ports.SaleStats
	SaleBadge     string // "%26'YA VARAN"

	Page any
}

func productURL(p domain.Product) string {
	return "/urun/" + url.PathEscape(p.ID) + "-" + p.Slug
}

func imgURL(path string) string {
	if path == "" {
		return ""
	}
	return "/img/" + path
}

func (s *Server) card(p domain.Product) cardVM {
	c := cardVM{
		ID:         p.ID,
		Name:       p.Title,
		URL:        productURL(p),
		ShopierURL: p.ShopierURL,
		Category:   p.PrimaryCategory().Title,
		Price:      p.Price.String(),
		HasOld:     p.HasDiscount(),
		IsNew:      s.catalog.IsNew(p),
		InStock:    p.InStock,
		Pct:        p.DiscountPercent(),
	}
	if c.HasOld {
		c.Old = p.OldPrice.String()
	}
	if c.ShopierURL == "" {
		c.ShopierURL = s.cfg.Site.ShopierStore
	}
	if img := p.MainImage(); img != nil {
		c.Img, c.Big = imgURL(img.ThumbPath), imgURL(img.Path)
	}
	return c
}

func (s *Server) cards(list []domain.Product) []cardVM {
	out := make([]cardVM, len(list))
	for i, p := range list {
		out[i] = s.card(p)
	}
	return out
}

func categoryURL(slug string) string {
	return "/kategori/" + url.PathEscape(slug)
}

func (s *Server) layout(title, desc, path string) layoutVM {
	sale := s.catalog.SaleStats()
	l := layoutVM{
		Site:          s.cfg.Site,
		InstagramURL:  s.cfg.InstagramURL(),
		Year:          s.now().Year(),
		Title:         title,
		Description:   desc,
		Canonical:     s.cfg.Site.BaseURL + path,
		OGImage:       s.cfg.Site.BaseURL + s.assetURL("img/logo.png"),
		OGType:        "website",
		Sale:          sale,
		CampaignOn:    s.cfg.Campaign.Active && sale.Count > 0,
		CampaignLabel: s.cfg.Campaign.Label,
	}
	if l.CampaignLabel == "" {
		l.CampaignLabel = "Kampanya"
	}
	if sale.MaxPercent > 0 {
		l.SaleBadge = "%" + strconv.Itoa(sale.MaxPercent) + "'" + domain.UpperTR(domain.DativeSuffix(sale.MaxPercent)) + " VARAN"
	}
	for _, c := range s.catalog.Categories() {
		l.Cats = append(l.Cats, catVM{Name: c.Title, Slug: c.Slug, URL: categoryURL(c.Slug), Count: c.Count})
	}
	return l
}

// ---- Ürün listesi filtre durumu ----

type listState struct {
	Cat       string
	Sale, New bool
	Q, Sort   string
}

func parseListState(q url.Values, pathCat string) listState {
	st := listState{
		Cat:  pathCat,
		Sale: q.Get("indirim") == "1",
		New:  q.Get("yeni") == "1",
		Q:    strings.TrimSpace(q.Get("q")),
		Sort: q.Get("sirala"),
	}
	if st.Cat == "" {
		st.Cat = q.Get("kategori")
	}
	if len(st.Q) > 100 {
		st.Q = st.Q[:100]
	}
	switch ports.Sort(st.Sort) {
	case ports.SortPriceAsc, ports.SortPriceDesc, ports.SortDiscount:
	default:
		st.Sort = ""
	}
	return st
}

// URL durumu temsil eden adres; yalnızca kategori seçiliyse /kategori/{slug}.
func (st listState) URL() string {
	q := url.Values{}
	if st.Sale {
		q.Set("indirim", "1")
	}
	if st.New {
		q.Set("yeni", "1")
	}
	if st.Q != "" {
		q.Set("q", st.Q)
	}
	if st.Sort != "" {
		q.Set("sirala", st.Sort)
	}
	if st.Cat != "" && len(q) == 0 {
		return categoryURL(st.Cat)
	}
	if st.Cat != "" {
		q.Set("kategori", st.Cat)
	}
	if len(q) == 0 {
		return "/urunler"
	}
	return "/urunler?" + q.Encode()
}

func (st listState) WithCat(slug string) string { st.Cat = slug; return st.URL() }
func (st listState) WithoutCat() string         { st.Cat = ""; return st.URL() }
func (st listState) WithoutSale() string        { st.Sale = false; return st.URL() }
func (st listState) WithoutNew() string         { st.New = false; return st.URL() }
func (st listState) WithoutQ() string           { st.Q = ""; return st.URL() }
func (st listState) FilterCount() int {
	n := 0
	if st.Sale {
		n++
	}
	if st.New {
		n++
	}
	return n
}

type chipVM struct{ Label, URL string }

type sortOption struct {
	Value, Label, Short string
	Selected            bool
}

type listVM struct {
	State      listState
	Title      string
	Count      int
	Items      []cardVM
	CatList    []catVM
	Total      int
	Chips      []chipVM
	Sorts      []sortOption
	CurrentCat *catVM
}

// ---- Ürün detay ----

type imageVM struct{ Full, Thumb string }

type productVM struct {
	Card         cardVM
	Savings      string
	Description  template.HTML
	Category     *catVM
	Images       []imageVM
	GalleryJSON  string
	Shipping     string
	StockText    string
	Similar      []cardVM
	ThumbClasses string
}

func galleryJSON(imgs []imageVM) string {
	urls := make([]string, len(imgs))
	for i, im := range imgs {
		urls[i] = im.Full
	}
	b, _ := json.Marshal(urls)
	return string(b)
}

// ---- Ana sayfa ----

type heroVM struct {
	Label, Title, Description, Button, ButtonURL string
	Prefix                                       string // büyük sayının önündeki "%" gibi
	BigNumber                                    int
	BadgeText, BadgeClass                        string
	Note1, Note2                                 string
}

type homeVM struct {
	Slides   []heroVM
	Featured []cardVM
	Sale     []cardVM
	Newest   []cardVM
}

// carouselVM kayan ürün şeridi.
type carouselVM struct {
	Label string
	Items []cardVM
}

func carousel(label string, items []cardVM) carouselVM { return carouselVM{Label: label, Items: items} }

type favoritesVM struct {
	Suggest []cardVM
}
