# Lider Otomarket

liderotomarket.com vitrin sitesi. Ürünler `urunler.json` dosyasından gelir, satın alma her zaman ürünün Shopier sayfasında olur.
Go ile yazıldı, hexagonal (ports & adapters) mimari; sayfalar sunucuda `html/template` ile üretilir.

## Ürünler: urunler.json

Shopier, ürün API'sini dışarıya kapattığı için ürün listesinin kaynağı proje kökündeki **`urunler.json`** dosyasıdır.
Shopier'de bir ürün eklediğinde, fiyatını değiştirdiğinde veya sildiğinde bu dosyayı da güncelle.

```json
{
  "id": "51853723",
  "name": "Volkswagen Passat Karbon Anahtar Kılıfı",
  "category": "Anahtarlık",
  "price": "600",
  "oldPrice": "750",
  "description": "Karbon görünümlü...\n\nBoş satır yeni paragraf olur.",
  "images": ["https://cdn.shopier.app/pictures_large/....png", "gorseller/kendi-fotografim.jpg"],
  "shippingPrice": "100",
  "inStock": true,
  "createdAt": "2026-11-11",
  "rank": 1
}
```

- **Zorunlu alanlar:** `id`, `name`, `price`.
- **`id`:** Shopier ürün linkinin sonundaki sayı, örneğin `shopier.com/liderotomarket/51853723`. "Shopier'de Al" butonu bununla doğru ürüne gider.
- **`price` / `oldPrice`:** TL olarak yazılır: `600`, `"1.199,90"` veya `1199.9`. İndirim yoksa `oldPrice` yazma.
- **`category`:** Tek kategori için kullanılır. Birden fazlaysa `"categories": ["Anahtarlık", "Sibop Kapağı"]` yaz.
- **`images`:** Shopier'deki görsel linkleri ya da proje klasörüne koyduğun fotoğrafların yolu olabilir. İlk görsel kapak olur.
- **`inStock: false`:** Ürün "Tükendi" olarak gösterilir.
- **`createdAt`:** "YENİ" rozeti için kullanılır. Hiçbir üründe yoksa listedeki en yeni 12 ürün "yeni" sayılır.
- **`rank`:** Sıralamadır, küçük olan önce gelir. Yazılmazsa dosyadaki sıra kullanılır.

Dosyada hata varsa (eksik fiyat, aynı id iki kez, bulunamayan görsel, yanlış yazılmış alan adı) `make guncelle` hatalı ürünü adıyla ve satır numarasıyla gösterir; site bozulmaz.

## Günlük kullanım

| Komut | Ne yapar |
|---|---|
| `make guncelle` | `urunler.json`'daki ürünleri ve görselleri `data/` klasörüne işler |
| `make deneme` | Dosyayı kontrol eder, kaydetmeden neyin değişeceğini gösterir |
| `make run` | Siteyi yerelde açar: http://localhost:8080 |
| `make yayinla` | Günceller ve Fly.io'ya yayınlar (tek komut) |
| `make ornek` | Tasarımdaki örnek ürünlerle siteyi açar |
| `make test` | Tüm testleri çalıştırır |

Kampanya bandı, öne çıkan ürünler, "yeni" rozeti süresi ve kategori sırası `config.yaml` dosyasından değiştirilir.
Kampanya bandı yalnızca indirimli ürün varsa kampanya metniyle gösterilir; yoksa `vitrin` metinleri kullanılır.

Shopier ürün API'si ileride açılırsa ürünler doğrudan Shopier'den de çekilebilir:
`.env` dosyasına `SHOPIER_PAT=...` yazıp `go run ./cmd/liderotomarket sync -kaynak shopier` komutunu çalıştır.

## Mimari

```
cmd/liderotomarket            serve | sync komutları, bağımlılıkların bağlandığı yer
internal/core/domain          Ürün, kategori, para (kuruş), slug, Türkçe arama
internal/core/ports           Arayüzler: ProductSource, ProductRepository, ImageStore, CatalogService, SyncService
internal/core/services        catalog (listeleme/filtre/sıralama), sync (urunler.json → SQLite + görseller)
internal/adapters/driven      jsonfile (urunler.json), shopier (REST API, şu an kapalı), sqlite (depo), imagestore (görsel indirme)
internal/adapters/driving     http (site), cli (senkron çıktısı)
web/templates, web/static     Şablonlar, CSS, JS, logolar (binary'e gömülür)
docs/tasarim                  Claude Design tasarım dosyaları (referans)
```

Favoriler tarayıcıda (`localStorage`) tutulur; sunucuda kalıcı veri yoktur. `data/` (SQLite + görseller) yerelde üretilip
Docker imajına gömülür, bu yüzden Fly.io'da disk (volume) gerekmez ve Shopier anahtarı sunucuya hiç gitmez.

## Yayın (Fly.io)

```sh
~/.fly/bin/flyctl auth login
~/.fly/bin/flyctl launch --no-deploy --copy-config   # ilk sefer: uygulamayı oluşturur
make yayinla
~/.fly/bin/flyctl certs add liderotomarket.com
~/.fly/bin/flyctl certs add www.liderotomarket.com
```
