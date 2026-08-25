package finance

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"unicode"
)

// NormalizeDescription lowercases, strips punctuation (unicode-aware), and
// collapses whitespace so trivial formatting differences do not defeat dedupe.
func NormalizeDescription(raw string) string {
	var b strings.Builder
	b.Grow(len(raw))
	lastSpace := true // leading spaces trimmed
	for _, r := range raw {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
			lastSpace = false
		default:
			if !lastSpace {
				b.WriteRune(' ')
				lastSpace = true
			}
		}
	}
	return strings.TrimRight(b.String(), " ")
}

// DescriptionHash is the dedupe hash: sha256 hex of the normalized description.
func DescriptionHash(raw string) string {
	sum := sha256.Sum256([]byte(NormalizeDescription(raw)))
	return hex.EncodeToString(sum[:])
}

// DedupeKey identifies a transaction natural key: date|amount|hash.
func DedupeKey(date string, amount int64, hash string) string {
	return date + "|" + itoa(amount) + "|" + hash
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
