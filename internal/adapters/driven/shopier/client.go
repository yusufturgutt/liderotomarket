// Package shopier Shopier REST API (https://api.shopier.com/v1) için ports.ProductSource uygulamasıdır.
// Kimlik doğrulama: Personal Access Token (PAT), "Authorization: Bearer <PAT>".
package shopier

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"liderotomarket/internal/core/domain"
	"liderotomarket/internal/core/ports"
)

const (
	DefaultBaseURL = "https://api.shopier.com/v1"
	pageLimit      = 50 // API üst sınırı
	maxPages       = 200
	maxAttempts    = 4
)

// Client Shopier API istemcisi.
type Client struct {
	baseURL   string
	token     string
	storeURL  string // ürün linki gelmezse kullanılır: https://www.shopier.com/liderotomarket
	http      *http.Client
	log       *slog.Logger
	pageDelay time.Duration
}

var _ ports.ProductSource = (*Client)(nil)

// Option istemci ayarı.
type Option func(*Client)

func WithBaseURL(u string) Option          { return func(c *Client) { c.baseURL = u } }
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.http = h } }
func WithLogger(l *slog.Logger) Option     { return func(c *Client) { c.log = l } }
func WithPageDelay(d time.Duration) Option { return func(c *Client) { c.pageDelay = d } }

// New istemci oluşturur.
func New(token, storeURL string, opts ...Option) (*Client, error) {
	if token == "" {
		return nil, errors.New("SHOPIER_PAT tanımlı değil: .env dosyasına Shopier kişisel erişim anahtarınızı ekleyin")
	}
	c := &Client{
		baseURL:   DefaultBaseURL,
		token:     token,
		storeURL:  storeURL,
		http:      &http.Client{Timeout: 30 * time.Second},
		log:       slog.Default(),
		pageDelay: 250 * time.Millisecond,
	}
	for _, o := range opts {
		o(c)
	}
	return c, nil
}

// FetchAll tüm sayfaları dolaşarak mağazadaki ürünleri döner.
func (c *Client) FetchAll(ctx context.Context) ([]domain.Product, error) {
	var out []domain.Product
	seen := map[string]bool{}
	for page := 1; page <= maxPages; page++ {
		items, totalPages, err := c.fetchPage(ctx, page)
		if err != nil {
			return nil, fmt.Errorf("sayfa %d: %w", page, err)
		}
		for _, it := range items {
			p := mapProduct(it, c.storeURL)
			if p.ID == "" || seen[p.ID] {
				continue
			}
			seen[p.ID] = true
			out = append(out, p)
		}
		c.log.Info("Shopier sayfası alındı", "sayfa", page, "toplam_sayfa", totalPages, "ürün", len(items))
		if len(items) == 0 || (totalPages > 0 && page >= totalPages) || (totalPages == 0 && len(items) < pageLimit) {
			return out, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(c.pageDelay):
		}
	}
	return out, fmt.Errorf("%d sayfadan fazla ürün var; sayfalama durdu", maxPages)
}

// StatusError API'nin başarısız yanıtı.
type StatusError struct {
	Code int
	Body string
}

func (e *StatusError) Error() string {
	switch e.Code {
	case http.StatusUnauthorized:
		return "Shopier anahtarı geçersiz veya süresi dolmuş (401). Panelden yeni bir Kişisel Erişim Anahtarı oluşturun"
	case http.StatusForbidden:
		return "Shopier anahtarının ürünleri okuma yetkisi yok (403)"
	}
	return fmt.Sprintf("Shopier API %d döndü: %s", e.Code, e.Body)
}

func (c *Client) fetchPage(ctx context.Context, page int) ([]apiProduct, int, error) {
	q := url.Values{}
	q.Set("page", strconv.Itoa(page))
	q.Set("limit", strconv.Itoa(pageLimit))
	q.Set("sort", "dateDesc")
	u := c.baseURL + "/products?" + q.Encode()

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, 0, err
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "liderotomarket-sync/1.0")

		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
		} else {
			body, readErr := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
			resp.Body.Close()
			switch {
			case readErr != nil:
				lastErr = readErr
			case resp.StatusCode == http.StatusOK:
				var items []apiProduct
				if err := json.Unmarshal(body, &items); err != nil {
					return nil, 0, fmt.Errorf("yanıt çözülemedi: %w", err)
				}
				total, _ := strconv.Atoi(resp.Header.Get("Shopier-Pagination-Total-Pages"))
				return items, total, nil
			case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
				lastErr = &StatusError{Code: resp.StatusCode, Body: truncate(string(body), 300)}
				if wait := retryAfter(resp.Header.Get("Retry-After")); wait > 0 {
					if err := sleep(ctx, wait); err != nil {
						return nil, 0, err
					}
					continue
				}
			default:
				return nil, 0, &StatusError{Code: resp.StatusCode, Body: truncate(string(body), 300)}
			}
		}
		if attempt < maxAttempts {
			if err := sleep(ctx, time.Duration(attempt*attempt)*500*time.Millisecond); err != nil {
				return nil, 0, err
			}
		}
	}
	return nil, 0, lastErr
}

func retryAfter(v string) time.Duration {
	if v == "" {
		return 0
	}
	if s, err := strconv.Atoi(v); err == nil && s >= 0 && s <= 120 {
		return time.Duration(s) * time.Second
	}
	return 0
}

func sleep(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
