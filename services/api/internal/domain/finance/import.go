package finance

// Draft is a transaction ready for insertion.
type Draft struct {
	Date        string
	Amount      int64
	RawDesc     string
	Merchant    string
	Hash        string
	CategoryID  string // may be empty (uncategorized)
}

// Result reports import outcome counts.
type Result struct {
	Imported        int        `json:"imported"`
	Skipped         int        `json:"skipped"` // duplicates by natural key
	SkippedInvalid  int        `json:"skipped_invalid"`
	AutoCategorized int        `json:"auto_categorized"`
	Errors          []RowError `json:"errors,omitempty"`
}

// Prepare dedupes parsed rows against existing keys and within the batch, and
// applies rules to assign categories. It is pure: callers persist drafts.
func Prepare(rows []TxnRow, rowErrs []RowError, existing map[string]struct{}, rules []Rule) ([]Draft, Result) {
	res := Result{Errors: rowErrs}
	res.SkippedInvalid = len(rowErrs)

	sorted := make([]Rule, len(rules))
	copy(sorted, rules)
	SortRules(sorted)

	drafts := make([]Draft, 0, len(rows))
	for _, r := range rows {
		h := DescriptionHash(r.RawDesc)
		key := DedupeKey(r.Date, r.Amount, h)
		if _, dup := existing[key]; dup {
			res.Skipped++
			continue
		}
		existing[key] = struct{}{} // in-file duplicates also count as skipped

		cat := ""
		if id, ok := Match(sorted, r.RawDesc); ok {
			cat = id
			res.AutoCategorized++
		}
		drafts = append(drafts, Draft{
			Date:       r.Date,
			Amount:     r.Amount,
			RawDesc:    r.RawDesc,
			Merchant:   r.Merchant,
			Hash:       h,
			CategoryID: cat,
		})
	}
	res.Imported = len(drafts)
	return drafts, res
}
