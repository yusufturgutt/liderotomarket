// Package config config.yaml ve ortam değişkenlerini (.env) okur.
package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Site marka bilgileri.
type Site struct {
	Name         string `yaml:"ad"`
	BaseURL      string `yaml:"adres"`
	ShopierStore string `yaml:"shopier_magaza"`
	Email        string `yaml:"eposta"`
	Instagram    string `yaml:"instagram"`
	Description  string `yaml:"aciklama"`
}

// Banner ana sayfa bandı metinleri.
type Banner struct {
	Active      bool   `yaml:"aktif"`
	Label       string `yaml:"etiket"`
	Title       string `yaml:"baslik"`
	Description string `yaml:"aciklama"`
	Until       string `yaml:"bitis"`
	Button      string `yaml:"buton"`
}

// Config uygulama ayarları.
type Config struct {
	Site          Site     `yaml:"site"`
	Campaign      Banner   `yaml:"kampanya"`
	Showcase      Banner   `yaml:"vitrin"`
	FeaturedIDs   []string `yaml:"one_cikanlar"`
	NewDays       int      `yaml:"yeni_urun_gun"`
	CategoryOrder []string `yaml:"kategori_sirasi"`

	ShopierToken string `yaml:"-"` // SHOPIER_PAT
}

// NewWindow "YENİ" rozeti süresi.
func (c Config) NewWindow() time.Duration {
	return time.Duration(c.NewDays) * 24 * time.Hour
}

// InstagramURL profil linki.
func (c Config) InstagramURL() string {
	return "https://www.instagram.com/" + strings.TrimPrefix(c.Site.Instagram, "@")
}

// Load config dosyasını okur; .env varsa ortam değişkenlerine yükler (var olanları ezmez).
func Load(path, envPath string) (Config, error) {
	if envPath != "" {
		if err := loadDotEnv(envPath); err != nil && !os.IsNotExist(err) {
			return Config{}, fmt.Errorf("%s okunamadı: %w", envPath, err)
		}
	}
	cfg := Config{NewDays: 30}
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("%s okunamadı: %w", path, err)
	}
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return cfg, fmt.Errorf("%s hatalı: %w", path, err)
	}
	for i, id := range cfg.FeaturedIDs {
		cfg.FeaturedIDs[i] = strings.TrimSpace(id)
	}
	cfg.Site.BaseURL = strings.TrimRight(cfg.Site.BaseURL, "/")
	if cfg.Site.Name == "" {
		cfg.Site.Name = "Lider Otomarket"
	}
	cfg.ShopierToken = strings.TrimSpace(os.Getenv("SHOPIER_PAT"))
	return cfg, nil
}

func loadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		if _, exists := os.LookupEnv(k); !exists {
			os.Setenv(k, v)
		}
	}
	return sc.Err()
}
