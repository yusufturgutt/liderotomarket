FLY ?= $(shell command -v flyctl 2>/dev/null || command -v fly 2>/dev/null || echo $(HOME)/.fly/bin/flyctl)

.PHONY: help guncelle deneme run ornek test build deploy yayinla

help: ## Komutları listeler
	@grep -E '^[a-z-]+:.*##' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  make %-10s %s\n", $$1, $$2}'

guncelle: ## urunler.json'daki ürünleri ve görselleri data/ klasörüne işler
	go run ./cmd/liderotomarket sync

deneme: ## urunler.json'u kontrol eder; kaydetmeden neyin değişeceğini gösterir
	go run ./cmd/liderotomarket sync -dry-run

run: ## Siteyi yerelde başlatır: http://localhost:8080
	go run ./cmd/liderotomarket serve

ornek: ## Shopier anahtarı olmadan, tasarımdaki örnek ürünlerle siteyi başlatır
	go run ./cmd/ornekveri && go run ./cmd/liderotomarket serve -data data-ornek

test: ## Tüm testleri çalıştırır
	go vet ./... && go test ./...

build: ## Linux için tek dosyalık program üretir: bin/liderotomarket
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/liderotomarket ./cmd/liderotomarket

deploy: ## Mevcut data/ klasörüyle siteyi Fly.io'ya yayınlar
	@test -f data/catalog.db || (echo "data/catalog.db yok: önce 'make guncelle' çalıştırın" && exit 1)
	$(FLY) deploy

yayinla: guncelle deploy ## urunler.json'dan günceller ve yayınlar (tek komut)
