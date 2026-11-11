package domain

import (
	"testing"
	"time"
)

func TestParseMoney(t *testing.T) {
	tests := []struct {
		in   string
		want Money
	}{
		{"1199", 119900},
		{"1199.9", 119990},
		{"1199.00", 119900},
		{"349.90", 34990},
		{"1.199,00", 119900},
		{"899,5", 89950},
		{"200 TL", 20000},
		{"", 0},
		{"0.00", 0},
	}
	for _, tt := range tests {
		got, err := ParseMoney(tt.in)
		if err != nil {
			t.Fatalf("ParseMoney(%q) hata: %v", tt.in, err)
		}
		if got != tt.want {
			t.Errorf("ParseMoney(%q) = %d, beklenen %d", tt.in, got, tt.want)
		}
	}
	if _, err := ParseMoney("abc"); err == nil {
		t.Error("geçersiz girdi hata vermeli")
	}
}

func TestMoneyString(t *testing.T) {
	tests := map[Money]string{
		0:         "0,00 TL",
		34990:     "349,90 TL",
		119900:    "1.199,00 TL",
		123456789: "1.234.567,89 TL",
	}
	for in, want := range tests {
		if got := in.String(); got != want {
			t.Errorf("%d.String() = %q, beklenen %q", in, got, want)
		}
	}
	if got := Money(119900).Decimal(); got != "1199.00" {
		t.Errorf("Decimal = %q", got)
	}
}

func TestSlugify(t *testing.T) {
	tests := map[string]string{
		"Deri Direksiyon Kılıfı – Siyah": "deri-direksiyon-kilifi-siyah",
		"Paspas & Bagaj":                 "paspas-ve-bagaj",
		"İç Aksesuar":                    "ic-aksesuar",
		"Cam Rüzgarlığı 4'lü Set":        "cam-ruzgarligi-4-lu-set",
		"ÇOK FONKSİYONLU 12V":            "cok-fonksiyonlu-12v",
		"!!!":                            "urun",
	}
	for in, want := range tests {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, beklenen %q", in, got, want)
		}
	}
}

func TestFold(t *testing.T) {
	if Fold("KILIF") != Fold("kılıf") || Fold("kılıf") != "kilif" {
		t.Errorf("Fold Türkçe karakterleri sadeleştirmeli: %q %q", Fold("KILIF"), Fold("kılıf"))
	}
	if Fold("İç Aksesuar") != "ic aksesuar" {
		t.Errorf("Fold(İç Aksesuar) = %q", Fold("İç Aksesuar"))
	}
}

func TestDativeSuffix(t *testing.T) {
	tests := map[int]string{26: "ya", 25: "e", 30: "a", 40: "a", 20: "ye", 50: "ye", 10: "a", 9: "a", 100: "e", 70: "e", 12: "ye"}
	for n, want := range tests {
		if got := DativeSuffix(n); got != want {
			t.Errorf("DativeSuffix(%d) = %q, beklenen %q", n, got, want)
		}
	}
}

func TestProductDiscount(t *testing.T) {
	p := Product{Price: 89900, OldPrice: 119900}
	if !p.HasDiscount() || p.DiscountPercent() != 25 || p.Savings() != 30000 {
		t.Errorf("indirim hesabı yanlış: %v %d %d", p.HasDiscount(), p.DiscountPercent(), p.Savings())
	}
	q := Product{Price: 50000}
	if q.HasDiscount() || q.DiscountPercent() != 0 || q.Savings() != 0 {
		t.Error("indirimsiz ürün indirimli görünmemeli")
	}
}

func TestProductIsNew(t *testing.T) {
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	p := Product{CreatedAt: now.Add(-10 * 24 * time.Hour)}
	if !p.IsNew(now, 30*24*time.Hour) {
		t.Error("10 günlük ürün yeni sayılmalı")
	}
	if p.IsNew(now, 5*24*time.Hour) {
		t.Error("5 günlük pencerede yeni sayılmamalı")
	}
	if (Product{}).IsNew(now, time.Hour) {
		t.Error("tarihsiz ürün yeni sayılmamalı")
	}
}
