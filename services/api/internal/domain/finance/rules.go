package finance

import "strings"

// Rule is a categorization rule: first match by (priority, id) wins.
type Rule struct {
	ID         string
	Pattern    string
	CategoryID string
	Priority   int
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

// Match returns the category of the first rule whose pattern is a
// case-insensitive substring of the description.
func Match(rules []Rule, description string) (string, bool) {
	desc := strings.ToLower(description)
	for _, r := range rules {
		if strings.Contains(desc, strings.ToLower(r.Pattern)) {
			return r.CategoryID, true
		}
	}
	return "", false
}
