package finance

import "testing"

func rules() []Rule {
	return []Rule{
		{ID: "r3", Pattern: "starbucks", CategoryID: "cat_coffee", Priority: 10},
		{ID: "r1", Pattern: "coffee", CategoryID: "cat_drinks", Priority: 5},
		{ID: "r2", Pattern: "SALARY", CategoryID: "cat_income", Priority: 100},
		{ID: "r4", Pattern: "zzz-never", CategoryID: "cat_none", Priority: 1},
	}
}

func TestSortRulesPriorityThenID(t *testing.T) {
	rs := rules()
	SortRules(rs)
	want := []string{"r4", "r1", "r3", "r2"}
	for i, id := range want {
		if rs[i].ID != id {
			t.Fatalf("pos %d = %s, want %s (got order %v)", i, rs[i].ID, id, ids(rs))
		}
	}
}

func TestMatchFirstByPriorityWins(t *testing.T) {
	rs := rules()
	SortRules(rs)
	cat, ok := Match(rs, "TRX STARBUCKS COFFEE 08/2026", -45000)
	if !ok || cat != "cat_drinks" {
		t.Fatalf("expected cat_drinks by priority 5 rule, got %q ok=%v", cat, ok)
	}
}

func TestMatchCaseInsensitive(t *testing.T) {
	rs := rules()
	SortRules(rs)
	if cat, ok := Match(rs, "monthly salary august", 12500000); !ok || cat != "cat_income" {
		t.Fatalf("case-insensitive match failed: %q %v", cat, ok)
	}
}

func TestMatchNoHit(t *testing.T) {
	rs := rules()
	SortRules(rs)
	if _, ok := Match(rs, "totally unrelated", -10000); ok {
		t.Fatal("expected no match")
	}
}

func ids(rs []Rule) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.ID
	}
	return out
}
