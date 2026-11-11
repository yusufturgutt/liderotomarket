// Package imagestore ürün görsellerini indirip yerel klasörde saklar (ports.ImageStore).
// Dosya adı kaynak URL'nin özetini içerir ({key}-{hash}.{uzantı}); böylece aynı görsel tekrar indirilmez
// ve değişen görsel yeni bir adla gelir (tarayıcı önbelleği güvenle "immutable" kalabilir).
package imagestore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"liderotomarket/internal/core/ports"
)

const maxImageBytes = 15 << 20

// Store dosya sistemi tabanlı görsel deposu. Eşzamanlı kullanım için güvenlidir.
type Store struct {
	dir  string
	http *http.Client
}

var _ ports.ImageStore = (*Store)(nil)

// New klasörü oluşturur.
func New(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Store{dir: dir, http: &http.Client{Timeout: 60 * time.Second}}, nil
}

func fileName(key, sourceURL, ext string) string {
	sum := sha256.Sum256([]byte(sourceURL))
	return fmt.Sprintf("%s-%s%s", key, hex.EncodeToString(sum[:])[:10], ext)
}

var allowedExt = map[string]bool{".webp": true, ".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".avif": true}

func extFromURL(u string) string {
	parsed, err := url.Parse(u)
	if err != nil {
		return ""
	}
	ext := strings.ToLower(path.Ext(parsed.Path))
	if allowedExt[ext] {
		return ext
	}
	return ""
}

// Save görseli indirir; aynı kaynaktan daha önce indirildiyse dokunmaz.
func (s *Store) Save(ctx context.Context, key, sourceURL string) (string, bool, error) {
	if strings.ContainsAny(key, `/\`) || key == "" {
		return "", false, fmt.Errorf("geçersiz anahtar %q", key)
	}
	// Uzantı URL'den belirlenebiliyorsa var olan dosyayı ağa çıkmadan bul.
	if ext := extFromURL(sourceURL); ext != "" {
		name := fileName(key, sourceURL, ext)
		if _, err := os.Stat(filepath.Join(s.dir, name)); err == nil {
			return name, false, nil
		}
	}

	if local, ok := strings.CutPrefix(sourceURL, "file://"); ok {
		return s.saveLocal(key, sourceURL, local)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return "", false, err
	}
	req.Header.Set("User-Agent", "liderotomarket-sync/1.0")
	resp, err := s.http.Do(req)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	ext := extFromURL(sourceURL)
	if ext == "" {
		ct, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
		switch ct {
		case "image/webp":
			ext = ".webp"
		case "image/jpeg":
			ext = ".jpg"
		case "image/png":
			ext = ".png"
		case "image/gif":
			ext = ".gif"
		case "image/avif":
			ext = ".avif"
		default:
			return "", false, fmt.Errorf("desteklenmeyen içerik türü %q", ct)
		}
	}
	name := fileName(key, sourceURL, ext)
	final := filepath.Join(s.dir, name)
	if _, err := os.Stat(final); err == nil {
		return name, false, nil
	}

	tmp, err := os.CreateTemp(s.dir, ".indiriliyor-*")
	if err != nil {
		return "", false, err
	}
	n, copyErr := io.Copy(tmp, io.LimitReader(resp.Body, maxImageBytes+1))
	closeErr := tmp.Close()
	if copyErr == nil && n > maxImageBytes {
		copyErr = fmt.Errorf("görsel çok büyük (>%d MB)", maxImageBytes>>20)
	}
	if copyErr == nil && n == 0 {
		copyErr = fmt.Errorf("boş yanıt")
	}
	if copyErr != nil || closeErr != nil {
		os.Remove(tmp.Name())
		if copyErr != nil {
			return "", false, copyErr
		}
		return "", false, closeErr
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		os.Remove(tmp.Name())
		return "", false, err
	}
	if err := os.Rename(tmp.Name(), final); err != nil {
		os.Remove(tmp.Name())
		return "", false, err
	}
	return name, true, nil
}

// saveLocal bilgisayardaki bir görseli depoya kopyalar. Dosya adı içeriğin özetinden üretilir;
// böylece fotoğraf değişirse yeni adla kopyalanır.
func (s *Store) saveLocal(key, sourceURL, local string) (string, bool, error) {
	ext := strings.ToLower(filepath.Ext(local))
	if ext == ".jpeg" {
		ext = ".jpg"
	}
	if !allowedExt[ext] {
		return "", false, fmt.Errorf("desteklenmeyen görsel türü %q (webp, jpg, png, gif, avif)", ext)
	}
	b, err := os.ReadFile(local)
	if err != nil {
		return "", false, err
	}
	if len(b) == 0 || len(b) > maxImageBytes {
		return "", false, fmt.Errorf("görsel boş veya çok büyük: %s", local)
	}
	sum := sha256.Sum256(b)
	name := fileName(key, sourceURL+hex.EncodeToString(sum[:]), ext)
	final := filepath.Join(s.dir, name)
	if _, err := os.Stat(final); err == nil {
		return name, false, nil
	}
	tmp := final + ".gecici"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return "", false, err
	}
	if err := os.Rename(tmp, final); err != nil {
		os.Remove(tmp)
		return "", false, err
	}
	return name, true, nil
}

// Prune keep listesinde olmayan dosyaları siler.
func (s *Store) Prune(keep map[string]bool) (int, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, e := range entries {
		if e.IsDir() || keep[e.Name()] {
			continue
		}
		if err := os.Remove(filepath.Join(s.dir, e.Name())); err != nil {
			return removed, err
		}
		removed++
	}
	return removed, nil
}
