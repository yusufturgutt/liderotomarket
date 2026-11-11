package httpserver

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"liderotomarket/internal/core/ports"
)

// stripSize kayan şeritlerde gösterilecek en fazla ürün sayısı.
const stripSize = 12

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	l := s.layout(s.cfg.Site.Name+" · Oto Aksesuar ve Parça", s.cfg.Site.Description, "/")
	l.Home = true

	all := s.catalog.All()
	featured := s.catalog.Featured(stripSize)
	vm := homeVM{
		Featured: s.cards(featured),
		Newest:   s.cards(s.catalog.Newest(stripSize)),
	}
	if l.CampaignOn {
		vm.Sale = s.cards(s.catalog.OnSale(stripSize))
	}
	vm.Slides = s.heroSlides(l, len(all), len(featured))
	l.JSONLD = jsonLD(map[string]any{
		"@context": "https://schema.org",
		"@type":    "Store",
		"name":     s.cfg.Site.Name,
		"url":      s.cfg.Site.BaseURL + "/",
		"logo":     l.OGImage,
		"email":    s.cfg.Site.Email,
		"sameAs":   []string{s.cfg.InstagramURL(), s.cfg.Site.ShopierStore},
	})
	l.Page = vm
	s.render(w, r, http.StatusOK, "anasayfa", l)
}

// heroSlides ana sayfa bandının slaytları: kampanya (veya vitrin), öne çıkanlar, yeni ürünler, kategoriler.
func (s *Server) heroSlides(l layoutVM, total, featured int) []heroVM {
	var slides []heroVM
	if l.CampaignOn {
		c := s.cfg.Campaign
		slides = append(slides, heroVM{Label: c.Label, Title: c.Title, Description: c.Description, Button: c.Button,
			ButtonURL: "/urunler?indirim=1", Prefix: "%", BigNumber: l.Sale.MaxPercent,
			BadgeText: fmt.Sprintf("%d ürün indirimde", l.Sale.Count), BadgeClass: "rozet-indirim",
			Note1: "en yüksek indirim", Note2: c.Until})
	} else {
		c := s.cfg.Showcase
		slides = append(slides, heroVM{Label: c.Label, Title: c.Title, Description: c.Description, Button: c.Button,
			ButtonURL: "/urunler", BigNumber: total, BadgeText: fmt.Sprintf("%d kategori", len(l.Cats)),
			BadgeClass: "rozet-indirim", Note1: "ürün", Note2: "Shopier güvencesiyle"})
	}
	if featured > 0 {
		slides = append(slides, heroVM{Label: "Seçtiklerimiz", Title: "Öne Çıkanlar",
			Description: "Mağazamızdan sizin için seçtiğimiz ürünler. Siparişler Shopier üzerinden.",
			Button:      "Öne çıkanları gör", ButtonURL: "#one-cikanlar", BigNumber: featured,
			BadgeText: "Seçtiklerimiz", BadgeClass: "rozet-mavi", Note1: "öne çıkan ürün", Note2: "Shopier güvencesiyle"})
	}
	if n := len(s.catalog.Search(ports.Query{OnlyNew: true})); n > 0 {
		slides = append(slides, heroVM{Label: "Mağazaya yeni geldi", Title: "Yeni Ürünler",
			Description: "Mağazamıza en son eklenen ürünler. Siparişler Shopier üzerinden.",
			Button:      "Yeni ürünleri gör", ButtonURL: "/urunler?yeni=1", BigNumber: n,
			BadgeText: "Yeni", BadgeClass: "rozet-mavi", Note1: "yeni ürün", Note2: "en son eklenenler"})
	}
	for _, c := range l.Cats {
		slides = append(slides, heroVM{Label: "Kategori", Title: c.Name,
			Description: c.Name + " kategorisindeki tüm ürünleri inceleyin. Siparişler Shopier üzerinden.",
			Button:      "Ürünleri gör", ButtonURL: c.URL, BigNumber: c.Count,
			BadgeText: "Shopier mağazasıyla aynı", BadgeClass: "rozet-mavi", Note1: "ürün", Note2: "bu kategoride"})
	}
	for i := range slides {
		if slides[i].Button == "" {
			slides[i].Button = "Ürünleri gör"
		}
	}
	return slides
}

var sortOptions = []sortOption{
	{Value: "", Label: "Önerilen", Short: "Önerilen"},
	{Value: string(ports.SortPriceAsc), Label: "Fiyat: düşükten yükseğe", Short: "Fiyat: artan"},
	{Value: string(ports.SortPriceDesc), Label: "Fiyat: yüksekten düşüğe", Short: "Fiyat: azalan"},
	{Value: string(ports.SortDiscount), Label: "İndirim oranı", Short: "İndirim oranı"},
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	st := parseListState(r.URL.Query(), r.PathValue("slug"))
	var current *catVM
	if st.Cat != "" {
		c, ok := s.catalog.CategoryBySlug(st.Cat)
		if !ok {
			s.handleNotFound(w, r)
			return
		}
		current = &catVM{Name: c.Title, Slug: c.Slug, URL: categoryURL(c.Slug), Count: c.Count}
	}
	// /urunler?kategori=x yalnızca kategori içeriyorsa kalıcı adrese yönlendir.
	if r.PathValue("slug") == "" && st.Cat != "" && st.URL() != r.URL.RequestURI() && st.URL() == categoryURL(st.Cat) {
		http.Redirect(w, r, st.URL(), http.StatusMovedPermanently)
		return
	}

	items := s.catalog.Search(ports.Query{CategorySlug: st.Cat, OnlySale: st.Sale, OnlyNew: st.New, Text: st.Q, Sort: ports.Sort(st.Sort)})
	l := s.layout("", "", "/urunler")
	l.SearchQuery = st.Q

	title := "Tüm Ürünler"
	switch {
	case current != nil:
		title = current.Name
	case st.Sale:
		title = l.CampaignLabel
	case st.New:
		title = "Yeni Ürünler"
	case st.Q != "":
		title = "“" + st.Q + "” için sonuçlar"
	}
	switch {
	case st.Sale && current == nil:
		l.Nav = "indirim"
	case st.New && !st.Sale && current == nil:
		l.Nav = "yeni"
	default:
		l.Nav = "urunler"
	}

	vm := listVM{State: st, Title: title, Count: len(items), Items: s.cards(items), CurrentCat: current, Total: len(s.catalog.All())}
	for _, c := range l.Cats {
		c.URL = st.WithCat(c.Slug)
		c.Current = c.Slug == st.Cat
		vm.CatList = append(vm.CatList, c)
	}
	if current != nil {
		vm.Chips = append(vm.Chips, chipVM{Label: current.Name, URL: st.WithoutCat()})
	}
	if st.Sale {
		vm.Chips = append(vm.Chips, chipVM{Label: "İndirimli", URL: st.WithoutSale()})
	}
	if st.New {
		vm.Chips = append(vm.Chips, chipVM{Label: "Yeni", URL: st.WithoutNew()})
	}
	if st.Q != "" {
		vm.Chips = append(vm.Chips, chipVM{Label: "“" + st.Q + "”", URL: st.WithoutQ()})
	}
	for _, o := range sortOptions {
		o.Selected = o.Value == st.Sort
		vm.Sorts = append(vm.Sorts, o)
	}

	l.Title = title + " · " + s.cfg.Site.Name
	l.Description = fmt.Sprintf("%s: %d ürün. %s", title, len(items), s.cfg.Site.Description)
	if current != nil {
		l.Canonical = s.cfg.Site.BaseURL + current.URL
	}
	// Filtrelenmiş/aranmış sonuçları arama motorlarına kapat (kategori ve tüm ürünler açık).
	if st.Q != "" || st.Sort != "" || st.Sale || st.New {
		l.NoIndex = true
	}
	l.Page = vm
	s.render(w, r, http.StatusOK, "urunler", l)
}

func (s *Server) handleProduct(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	id, _, _ := strings.Cut(key, "-")
	p, ok := s.catalog.ByID(id)
	if !ok {
		s.handleNotFound(w, r)
		return
	}
	canonical := productURL(p)
	if "/urun/"+key != canonical {
		http.Redirect(w, r, canonical, http.StatusMovedPermanently)
		return
	}
	card := s.card(p)
	vm := productVM{Card: card, Description: template.HTML(p.DescriptionHTML), Similar: s.cards(s.catalog.Similar(p, stripSize))}
	if p.HasDiscount() {
		vm.Savings = p.Savings().String() + " tasarruf"
	}
	if c := p.PrimaryCategory(); c.Slug != "" {
		vm.Category = &catVM{Name: c.Title, Slug: c.Slug, URL: categoryURL(c.Slug)}
	}
	for _, img := range p.Images {
		vm.Images = append(vm.Images, imageVM{Full: imgURL(img.Path), Thumb: imgURL(img.ThumbPath)})
	}
	vm.GalleryJSON = galleryJSON(vm.Images)
	vm.ThumbClasses = "adet-" + strconv.Itoa(max(4, min(len(vm.Images), 5)))
	if p.ShippingPrice > 0 {
		vm.Shipping = p.ShippingPrice.String()
	} else {
		vm.Shipping = "Shopier sipariş sayfasında belirtilir"
	}
	vm.StockText = "Stokta"
	if !p.InStock {
		vm.StockText = "Tükendi"
	}

	desc := p.DescriptionText
	if desc == "" {
		desc = p.Title + " — " + p.Price.String() + ". Siparişinizi Shopier üzerinden güvenle verin."
	}
	if r := []rune(desc); len(r) > 160 {
		desc = strings.TrimSpace(string(r[:157])) + "…"
	}
	l := s.layout(p.Title+" · "+s.cfg.Site.Name, desc, canonical)
	l.OGType = "product"
	var images []string
	for _, im := range vm.Images {
		images = append(images, s.cfg.Site.BaseURL+im.Full)
	}
	if len(images) > 0 {
		l.OGImage = images[0]
	}
	availability := "https://schema.org/InStock"
	if !p.InStock {
		availability = "https://schema.org/OutOfStock"
	}
	currency := p.Currency
	if currency == "" {
		currency = "TRY"
	}
	ld := map[string]any{
		"@context":    "https://schema.org",
		"@type":       "Product",
		"name":        p.Title,
		"sku":         p.ID,
		"url":         l.Canonical,
		"description": p.DescriptionText,
		"brand":       map[string]any{"@type": "Brand", "name": s.cfg.Site.Name},
		"offers": map[string]any{
			"@type": "Offer", "url": card.ShopierURL, "priceCurrency": currency,
			"price": p.Price.Decimal(), "availability": availability,
		},
	}
	if len(images) > 0 {
		ld["image"] = images
	}
	if vm.Category != nil {
		ld["category"] = vm.Category.Name
	}
	l.JSONLD = jsonLD(ld)
	l.Page = vm
	s.render(w, r, http.StatusOK, "urun", l)
}

func (s *Server) handleFavorites(w http.ResponseWriter, r *http.Request) {
	l := s.layout("Favorilerim · "+s.cfg.Site.Name, "Favori ürünleriniz bu tarayıcıda saklanır.", "/favoriler")
	l.Nav, l.NoIndex = "favoriler", true
	l.Page = favoritesVM{Suggest: s.cards(s.catalog.Featured(stripSize))}
	s.render(w, r, http.StatusOK, "favoriler", l)
}

// handleFavoritesFragment favoriler sayfası için satırları HTML parçası olarak döner.
func (s *Server) handleFavoritesFragment(w http.ResponseWriter, r *http.Request) {
	var ids []string
	for _, id := range strings.Split(r.URL.Query().Get("ids"), ",") {
		if id = strings.TrimSpace(id); id != "" && len(ids) < 200 {
			ids = append(ids, id)
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Robots-Tag", "noindex")
	if err := s.pages["parca-favoriler"].ExecuteTemplate(w, "favori-satirlar", s.cards(s.catalog.ByIDs(ids))); err != nil {
		s.log.Error("favori parçası", "hata", err)
	}
}

// handleSearchFragment yazarken açılan arama sonuçlarını (en fazla 6 ürün) HTML parçası olarak döner.
func (s *Server) handleSearchFragment(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(q)) > 100 {
		q = string([]rune(q)[:100])
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=60")
	w.Header().Set("X-Robots-Tag", "noindex")
	if len([]rune(q)) < 2 {
		return
	}
	found := s.catalog.Search(ports.Query{Text: q})
	data := struct {
		Q      string
		Items  []cardVM
		Total  int
		AllURL string
	}{Q: q, Items: s.cards(found[:min(6, len(found))]), Total: len(found), AllURL: listState{Q: q}.URL()}
	if err := s.pages["parca-arama"].ExecuteTemplate(w, "arama-sonuclari", data); err != nil {
		s.log.Error("arama parçası", "hata", err)
	}
}

func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	l := s.layout("Sayfa bulunamadı · "+s.cfg.Site.Name, "", r.URL.Path)
	l.NoIndex = true
	l.Page = favoritesVM{Suggest: s.cards(s.catalog.Featured(stripSize))}
	s.render(w, r, http.StatusNotFound, "bulunamadi", l)
}

func (s *Server) handleRobots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, "User-agent: *\nDisallow: /parca/\nAllow: /\n\nSitemap: %s/sitemap.xml\n", s.cfg.Site.BaseURL)
}

type sitemapURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

func (s *Server) handleSitemap(w http.ResponseWriter, r *http.Request) {
	base := s.cfg.Site.BaseURL
	last := s.catalog.LastSync()
	lm := func(t time.Time) string {
		if t.IsZero() {
			return ""
		}
		return t.UTC().Format("2006-01-02")
	}
	urls := []sitemapURL{{Loc: base + "/", LastMod: lm(last)}, {Loc: base + "/urunler", LastMod: lm(last)}}
	for _, c := range s.catalog.Categories() {
		urls = append(urls, sitemapURL{Loc: base + categoryURL(c.Slug), LastMod: lm(last)})
	}
	for _, p := range s.catalog.All() {
		t := p.UpdatedAt
		if t.IsZero() {
			t = last
		}
		urls = append(urls, sitemapURL{Loc: base + productURL(p), LastMod: lm(t)})
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Write([]byte(xml.Header))
	enc := xml.NewEncoder(w)
	enc.Encode(struct {
		XMLName xml.Name     `xml:"urlset"`
		NS      string       `xml:"xmlns,attr"`
		URLs    []sitemapURL `xml:"url"`
	}{NS: "http://www.sitemaps.org/schemas/sitemap/0.9", URLs: urls})
}

func jsonLD(v any) template.JS {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	// </script> kaçışı json.Marshal tarafından < olarak yapılır.
	return template.JS(b)
}
