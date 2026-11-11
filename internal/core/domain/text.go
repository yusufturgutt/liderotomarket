package domain

import (
	"strings"
	"unicode"
)

var trFold = strings.NewReplacer(
	"ç", "c", "Ç", "c",
	"ğ", "g", "Ğ", "g",
	"ı", "i", "I", "i", "İ", "i", "i̇", "i",
	"ö", "o", "Ö", "o",
	"ş", "s", "Ş", "s",
	"ü", "u", "Ü", "u",
	"â", "a", "Â", "a", "î", "i", "û", "u",
)

// Fold aramada kullanılmak üzere metni küçük harfe çevirir ve Türkçe karakterleri sadeleştirir.
// "Kılıf" ve "kilif" aynı sonucu verir.
func Fold(s string) string {
	s = trFold.Replace(s)
	return strings.ToLower(s)
}

// Slugify "Deri Direksiyon Kılıfı – Siyah" → "deri-direksiyon-kilifi-siyah".
func Slugify(s string) string {
	s = Fold(s)
	var b strings.Builder
	dash := false
	for _, r := range s {
		switch {
		case r == '&':
			if b.Len() > 0 && !dash {
				b.WriteByte('-')
			}
			b.WriteString("ve")
			dash = false
		case r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			b.WriteRune(r)
			dash = false
		default:
			if b.Len() > 0 && !dash {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 80 {
		out = strings.Trim(out[:80], "-")
	}
	if out == "" {
		out = "urun"
	}
	return out
}

// UpperTR Türkçe kurallarıyla büyük harfe çevirir (i → İ).
func UpperTR(s string) string {
	return strings.ToUpperSpecial(unicode.TurkishCase, s)
}

// DativeSuffix "%26'ya", "%30'a" gibi yönelme eki: sayının okunuşuna göre 'a/'e/'ya/'ye.
func DativeSuffix(n int) string {
	if n < 0 {
		n = -n
	}
	ones := map[int]string{1: "e", 2: "ye", 3: "e", 4: "e", 5: "e", 6: "ya", 7: "ye", 8: "e", 9: "a"}
	tens := map[int]string{1: "a", 2: "ye", 3: "a", 4: "a", 5: "ye", 6: "a", 7: "e", 8: "e", 9: "a"}
	switch {
	case n == 0:
		return "a" // sıfır'a
	case n%10 != 0:
		return ones[n%10]
	case n%100 != 0:
		return tens[(n/10)%10]
	case n%1000 != 0:
		return "e" // yüz'e
	default:
		return "e" // bin'e
	}
}
