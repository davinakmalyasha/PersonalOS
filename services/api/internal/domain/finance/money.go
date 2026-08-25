package finance

import (
	"fmt"
	"strings"
)

// ParseAmount converts a bank-export amount string into signed integer minor
// units. Handles currency symbols/codes, thousands separators in both
// conventions (1,234.56 and 1.234,56), spaces, and parentheses negatives.
func ParseAmount(s string) (int64, error) {
	t := strings.TrimSpace(s)
	if t == "" {
		return 0, fmt.Errorf("empty amount")
	}

	neg := false
	if strings.Contains(t, "(") && strings.Contains(t, ")") {
		neg = true
	}
	t = strings.Map(func(r rune) rune {
		switch {
		case r >= '0' && r <= '9':
			return r
		case r == '.' || r == ',' || r == '-':
			return r
		default:
			return -1 // strips Rp, $, IDR, spaces, NBSP, etc.
		}
	}, t)
	if strings.HasPrefix(strings.TrimSpace(t), "-") || neg {
		neg = true
	}

	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, t)
	if digits == "" {
		return 0, fmt.Errorf("no digits in %q", s)
	}

	lastDot := strings.LastIndex(t, ".")
	lastComma := strings.LastIndex(t, ",")

	var intPart, fracPart string
	switch {
	case lastDot >= 0 && lastComma >= 0:
		// Both present: the rightmost is the decimal separator.
		if lastComma > lastDot {
			intPart = strings.ReplaceAll(t[:lastComma], ".", "")
			intPart = strings.ReplaceAll(intPart, ",", "")
			fracPart = t[lastComma+1:]
		} else {
			intPart = strings.ReplaceAll(t[:lastDot], ",", "")
			intPart = strings.ReplaceAll(intPart, ".", "")
			fracPart = t[lastDot+1:]
		}
	case lastComma >= 0:
		intPart, fracPart = splitAmbiguous(t, ',')
	case lastDot >= 0:
		intPart, fracPart = splitAmbiguous(t, '.')
	default:
		intPart = t
	}

	fracPart = strings.TrimSuffix(fracPart, ".")
	if len(fracPart) > 2 {
		// >2 fractional digits: keep first two (bankers rarely need more here).
		fracPart = fracPart[:2]
	}
	for len(fracPart) < 2 {
		fracPart += "0"
	}

	var value int64
	for _, r := range intPart {
		if r < '0' || r > '9' {
			continue
		}
		value = value*10 + int64(r-'0')
	}
	frac := int64(0)
	if len(fracPart) >= 1 {
		frac += int64(fracPart[0]-'0') * 10
	}
	if len(fracPart) >= 2 {
		frac += int64(fracPart[1] - '0')
	}
	value = value*100 + frac

	if neg {
		value = -value
	}
	return value, nil
}

// splitAmbiguous decides whether the single separator is decimal or thousands.
// Rule: a pure-digit tail of exactly three digits after the separator is
// thousands ("1,234" US / "125.000" ID); any other tail is a decimal fraction
// ("12,34" / "12.34"). Files needing different semantics can pre-normalize.
func splitAmbiguous(t string, sep rune) (string, string) {
	idx := strings.LastIndex(t, string(sep))
	if idx < 0 {
		return stripOthers(t, "0123456789"), ""
	}
	intPart := stripOthers(t[:idx], "0123456789")
	tail := stripOthers(t[idx+1:], "0123456789")
	if len(tail) == 3 {
		return intPart + tail, ""
	}
	return intPart, tail
}

func stripOthers(s, keep string) string {
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune(keep, r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
