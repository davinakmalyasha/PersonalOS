package finance

import "strings"

// Rule is a categorization rule: first match by (priority, id) wins.
// AmountMin/AmountMax (when non-nil) bound the transaction's ABSOLUTE amount,
// so "min 1M" catches a -1,000,000 spend or a +1,000,000 deposit alike.
type Rule struct {
	ID         string
	Pattern    string
	CategoryID string
	Priority   int
	AmountMin  *int64
	AmountMax  *int64
}

// SortRules orders rules for evaluation: priority ascending, then id for
// deterministic tie-breaking.
func SortRules(rules []Rule) {
	for i := 1; i < len(rules); i++ {
		for j := i; j > 0; j-- {
			a, b := rules[j-1], rules[j]
			if a.Priority > b.Priority || (a.Priority == b.Priority && a.ID > b.ID) {
				rules[j-1], rules[j] = b, a
			} else {
				break
			}
		}
	}
}

// Abs returns |n| without overflowing on MinInt64 edge cases users won't hit.
func Abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

// Match returns the category of the first rule whose pattern is a
// case-insensitive substring of the description and whose optional amount
// window contains |amount|.
func Match(rules []Rule, description string, amount int64) (string, bool) {
	desc := strings.ToLower(description)
	mag := Abs(amount)
	for _, r := range rules {
		if !strings.Contains(desc, strings.ToLower(r.Pattern)) {
			continue
		}
		if r.AmountMin != nil && mag < *r.AmountMin {
			continue
		}
		if r.AmountMax != nil && mag > *r.AmountMax {
			continue
		}
		return r.CategoryID, true
	}
	return "", false
}
