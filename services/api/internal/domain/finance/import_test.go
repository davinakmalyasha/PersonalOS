package finance

import (
	"strings"
	"testing"
)

func existingKeys(rows ...TxnRow) map[string]struct{} {
	m := make(map[string]struct{})
	for _, r := range rows {
		m[DedupeKey(r.Date, r.Amount, DescriptionHash(r.RawDesc))] = struct{}{}
	}
	return m
}

func TestPrepareFirstImportCategorizes(t *testing.T) {
	csvRows := `Date,Description,Amount
2026-08-01,STARBUCKS COFFEE,-125.00
2026-08-02,SALARY AUGUST,5000.00
`
	rows, errs, _, _, err := ParseCSV(strings.NewReader(csvRows), nil, "")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	ruleList := []Rule{{ID: "r1", Pattern: "starbucks", CategoryID: "c1", Priority: 1}}

	drafts, res := Prepare(rows, errs, map[string]struct{}{}, ruleList)
	if res.Imported != 2 || res.Skipped != 0 || len(res.Errors) != 0 {
		t.Fatalf("res = %+v", res)
	}
	if res.AutoCategorized != 1 {
		t.Fatalf("expected 1 auto-categorized, got %d", res.AutoCategorized)
	}
	if drafts[0].CategoryID != "c1" {
		t.Fatalf("draft0 category = %q", drafts[0].CategoryID)
	}
	if drafts[1].CategoryID != "" {
		t.Fatalf("salary should be uncategorized, got %q", drafts[1].CategoryID)
	}
}

func TestPrepareSecondImportZeroDuplicates(t *testing.T) {
	csvRows := `Date,Description,Amount
2026-08-01,STARBUCKS COFFEE,-125.00
2026-08-02,SALARY AUGUST,5000.00
2026-08-03,TOKO ABC,250.00
`
	rows, errs, _, _, _ := ParseCSV(strings.NewReader(csvRows), nil, "")
	_, firstRes := Prepare(rows, errs, map[string]struct{}{}, nil)
	if firstRes.Imported != 3 {
		t.Fatalf("first import: %+v", firstRes)
	}

	// Simulate DB state from first run.
	existing := existingKeys(rows...)

	rows2, errs2, _, _, _ := ParseCSV(strings.NewReader(csvRows), nil, "")
	_, secondRes := Prepare(rows2, errs2, existing, nil)
	if secondRes.Imported != 0 {
		t.Fatalf("acceptance violated: second import imported %d", secondRes.Imported)
	}
	if secondRes.Skipped != 3 {
		t.Fatalf("second import skipped = %d, want 3", secondRes.Skipped)
	}
}

func TestPrepareInFileDuplicatesSkipped(t *testing.T) {
	csvRows := `Date,Description,Amount
2026-08-01,TOKO ABC,100.00
2026-08-01,toko abc ,100.00
`
	rows, _, _, _, _ := ParseCSV(strings.NewReader(csvRows), nil, "")
	// Same natural key after normalization â†’ second is in-file duplicate.
	if NormalizeDescription("toko abc ") != NormalizeDescription("TOKO ABC") {
		t.Fatal("precondition failed")
	}
	_, res := Prepare(rows, nil, map[string]struct{}{}, nil)
	if res.Imported != 1 || res.Skipped != 1 {
		t.Fatalf("in-file dedupe failed: %+v", res)
	}
}

func TestPrepareInvalidRowsCounted(t *testing.T) {
	bad := `Date,Description,Amount
not-a-date,BROKEN,10.00
2026-08-02,FINE,5.00
`
	rows, errs, _, _, err := ParseCSV(strings.NewReader(bad), nil, "")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(errs) != 1 {
		t.Fatalf("errs = %+v", errs)
	}
	_, res := Prepare(rows, errs, map[string]struct{}{}, nil)
	if res.SkippedInvalid != 1 || res.Imported != 1 {
		t.Fatalf("res = %+v", res)
	}
}
