package httpserver

import (
	"fmt"
	"html/template"
)

// Tasarımdaki çizgi ikonlar (stroke 1.75, köşeli uçlar).
var iconPaths = map[string]string{
	"kalp":      `<path d="M12 20s-7.5-4.6-7.5-10.2A4.3 4.3 0 0 1 12 7.2a4.3 4.3 0 0 1 7.5 2.6C19.5 15.4 12 20 12 20z"></path>`,
	"dis-link":  `<path d="M7 17L17 7M9 7h8v8"></path>`,
	"ok":        `<path d="M5 12h14M13 6l6 6-6 6"></path>`,
	"ara":       `<circle cx="11" cy="11" r="7"></circle><path d="M20 20l-4-4"></path>`,
	"instagram": `<rect x="3" y="3" width="18" height="18" rx="5"></rect><circle cx="12" cy="12" r="4"></circle><path d="M17.5 6.5h.01"></path>`,
	"zarf":      `<rect x="3" y="5" width="18" height="14"></rect><path d="M3 6l9 7 9-7"></path>`,
	"canta":     `<path d="M4 8h16l-1 12H5L4 8z"></path><path d="M9 8V6a3 3 0 0 1 6 0v2"></path>`,
	"menu":      `<path d="M4 7h16M4 12h16M4 17h16"></path>`,
	"kapat":     `<path d="M6 6l12 12M18 6L6 18"></path>`,
	"asagi":     `<path d="M6 9l6 6 6-6"></path>`,
	"sag":       `<path d="M9 6l6 6-6 6"></path>`,
	"sol":       `<path d="M15 6l-6 6 6 6"></path>`,
	"filtre":    `<path d="M4 6h16M7 12h10M10 18h4"></path>`,
	"gorsel":    `<rect x="3" y="4" width="18" height="16"></rect><circle cx="9" cy="10" r="2"></circle><path d="M3 18l6-5 4 3 3-2 5 4"></path>`,
}

// icon satır içi SVG üretir. Görsel yer tutucu ince çizgiyle ve gri renkte çizilir.
func icon(name string, size int) template.HTML {
	p, ok := iconPaths[name]
	if !ok {
		return ""
	}
	stroke, width, cap := "currentColor", "1.75", ` stroke-linecap="square"`
	switch name {
	case "kalp", "instagram", "zarf", "canta":
		cap = ""
	case "gorsel":
		stroke, cap = "#8A909C", ""
		width = "1"
		if size >= 72 {
			width = "0.8"
		}
	case "kapat":
		if size <= 16 {
			width = "2"
		}
	}
	return template.HTML(fmt.Sprintf(
		`<svg width="%d" height="%d" viewBox="0 0 24 24" fill="none" stroke="%s" stroke-width="%s"%s aria-hidden="true" focusable="false">%s</svg>`,
		size, size, stroke, width, cap, p))
}
