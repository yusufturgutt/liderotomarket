package domain

import (
	"fmt"
	"strconv"
	"strings"
)

// Money kuruş cinsinden tutar. 899,00 TL = 89900.
type Money int64

// ParseMoney Shopier'in string fiyatlarını çözer: "1199", "1199.9", "1199.00", "1.199,00".
func ParseMoney(s string) (Money, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(strings.TrimSuffix(s, "TL"), "₺")
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	// Türkçe biçim: binlik nokta, ondalık virgül.
	if strings.Contains(s, ",") {
		s = strings.ReplaceAll(s, ".", "")
		s = strings.ReplaceAll(s, ",", ".")
	}
	whole, frac, _ := strings.Cut(s, ".")
	if whole == "" {
		whole = "0"
	}
	w, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("geçersiz fiyat %q: %w", s, err)
	}
	if len(frac) > 2 {
		frac = frac[:2]
	}
	for len(frac) < 2 {
		frac += "0"
	}
	f, err := strconv.ParseInt(frac, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("geçersiz fiyat %q: %w", s, err)
	}
	if w < 0 {
		return 0, fmt.Errorf("negatif fiyat %q", s)
	}
	return Money(w*100 + f), nil
}

// String "1.199,00 TL" biçiminde yazar.
func (m Money) String() string {
	neg := m < 0
	if neg {
		m = -m
	}
	whole := int64(m) / 100
	frac := int64(m) % 100
	digits := strconv.FormatInt(whole, 10)
	var b strings.Builder
	if neg {
		b.WriteByte('-')
	}
	for i, r := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(r)
	}
	fmt.Fprintf(&b, ",%02d TL", frac)
	return b.String()
}

// Decimal schema.org için "1199.00" biçimi.
func (m Money) Decimal() string {
	return fmt.Sprintf("%d.%02d", int64(m)/100, int64(m)%100)
}
