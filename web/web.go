// Package web şablonları ve statik dosyaları binary'e gömer.
package web

import "embed"

//go:embed templates static
var FS embed.FS
