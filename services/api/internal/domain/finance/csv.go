package finance

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ColumnMapping holds resolved column indexes for a bank CSV export.
type ColumnMapping struct {
	Date        int
	Description int
	Amount      int // single-column mode
	Debit       int // split-column mode (money out)
	Credit      int // split-column mode (money in)
	Merchant    int // optional; -1 when absent
}

// HeaderSynonyms covers common English + Indonesian bank export headers.
var headerSynonyms = map[string][]string{
	"date":        {"date", "transaction date", "posting date", "value date", "tanggal", "tanggal transaksi", "tanggal nilai"},
	"description": {"description", "memo", "narrative", "details", "transaction details", "remark", "remarks", "keterangan", "deskripsi", "uraian", "keterangan transaksi"},
	"amount":      {"amount", "jumlah", "nilai", "nominal", "mutation amount", "jumlah mutasi"},
	"debit":       {"debit", "withdrawal", "money out", "debit amount", "beban"},
	"credit":      {"credit", "deposit", "money in", "credit amount", "kredit", "saldo masuk"},
	"merchant":    {"merchant", "counterparty", "pihak"},
}

func normHeader(s string) string {
	return strings.ToLower(strings.TrimSpace(strings.Join(strings.Fields(s), " ")))
}

func findColumn(headers []string, synonyms []string, taken map[int]bool) int {
	for _, syn := range synonyms {
		for i, h := range headers {
			if !taken[i] && h == syn {
				taken[i] = true
				return i
			}
		}
	}
	for _, syn := range synonyms {
		for i, h := range headers {
			if !taken[i] && strings.Contains(h, syn) {
				taken[i] = true
				return i
			}
		}
	}
	return -1
}

// DetectMapping maps header names to columns. Returns an error when required
// roles (date + description + one of amount|debit|credit) cannot be resolved.
func DetectMapping(header []string) (*ColumnMapping, error) {
	norm := make([]string, len(header))
	for i, h := range header {
		norm[i] = normHeader(h)
	}
	taken := map[int]bool{}
	m := &ColumnMapping{Merchant: -1}
	m.Date = findColumn(norm, headerSynonyms["date"], taken)
	m.Description = findColumn(norm, headerSynonyms["description"], taken)
	m.Amount = findColumn(norm, headerSynonyms["amount"], taken)
	m.Debit = findColumn(norm, headerSynonyms["debit"], taken)
	m.Credit = findColumn(norm, headerSynonyms["credit"], taken)
	m.Merchant = findColumn(norm, headerSynonyms["merchant"], taken)
	if m.Date < 0 || m.Description < 0 || (m.Amount < 0 && m.Debit < 0 && m.Credit < 0) {
		return nil, fmt.Errorf("could not auto-detect columns from header %v (pass explicit mapping)", header)
	}
	return m, nil
}

// TxnRow is one parsed, normalized transaction candidate.
type TxnRow struct {
	Date        string // YYYY-MM-DD
	Amount      int64  // signed minor units: spend negative, income positive
	RawDesc     string
	Merchant    string
}

type RowError struct {
	Line    int    `json:"line"`
	Message string `json:"message"`
}

// ParseCSV reads the whole file: detects the separator, finds the header,
// resolves the mapping (explicit overrides win), and parses rows. Rows with
// bad cells are collected as errors and skipped rather than failing the batch.
func ParseCSV(r io.Reader, override *ColumnMapping, dateFormat string) ([]TxnRow, []RowError, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, err
	}
	trimmed := strings.TrimLeft(string(raw), "\ufeff")
	if strings.TrimSpace(trimmed) == "" {
		return nil, nil, fmt.Errorf("empty CSV")
	}
	sep := detectSeparator(trimmed)

	cr := csv.NewReader(strings.NewReader(trimmed))
	cr.Comma = sep
	cr.FieldsPerRecord = -1
	cr.LazyQuotes = true
	cr.TrimLeadingSpace = true

	records, err := cr.ReadAll()
	if err != nil {
		return nil, nil, fmt.Errorf("read csv: %w", err)
	}
	if len(records) < 2 {
		return nil, nil, fmt.Errorf("csv needs a header row plus at least one data row")
	}

	mapping := override
	if mapping == nil {
		mapping, err = DetectMapping(records[0])
		if err != nil {
			return nil, nil, err
		}
	}

	// Sniff date layout across all rows for consistency.
	var dateCells []string
	for _, rec := range records[1:] {
		if c := cell(rec, mapping.Date); c != "" {
			dateCells = append(dateCells, c)
		}
	}
	layout := dateFormat
	if layout == "" {
		layout = SniffDateLayout(dateCells)
		if layout == "" {
			return nil, nil, fmt.Errorf("could not determine date format from samples (pass date_format)")
		}
	}

	var rows []TxnRow
	var rowErrs []RowError
	for i, rec := range records[1:] {
		line := i + 2 // 1-based incl header
		if isBlankRow(rec) {
			continue
		}
		row, err := parseRow(rec, mapping, layout)
		if err != nil {
			rowErrs = append(rowErrs, RowError{Line: line, Message: err.Error()})
			continue
		}
		rows = append(rows, row)
	}
	return rows, rowErrs, nil
}

func parseRow(rec []string, m *ColumnMapping, layout string) (TxnRow, error) {
	dateCell := cell(rec, m.Date)
	descCell := cell(rec, m.Description)
	date, err := ParseDate(dateCell, layout)
	if err != nil {
		return TxnRow{}, err
	}
	if descCell == "" {
		return TxnRow{}, fmt.Errorf("empty description")
	}

	var amount int64
	switch {
	case m.Amount >= 0:
		amount, err = ParseAmount(cell(rec, m.Amount))
	case m.Debit >= 0 && m.Credit >= 0:
		var debit, credit int64
		debit, errDebit := ParseAmount(orZero(cell(rec, m.Debit)))
		credit, errCredit := ParseAmount(orZero(cell(rec, m.Credit)))
		if errDebit != nil && errCredit != nil {
			return TxnRow{}, fmt.Errorf("no amount in debit/credit cells")
		}
		err = nil
		amount = credit - debit
	default:
		err = fmt.Errorf("mapping has neither amount nor debit/credit columns")
	}
	if err != nil {
		return TxnRow{}, err
	}
	if amount == 0 {
		return TxnRow{}, fmt.Errorf("zero amount")
	}

	row := TxnRow{Date: date, Amount: amount, RawDesc: descCell}
	if m.Merchant >= 0 {
		row.Merchant = strings.TrimSpace(cell(rec, m.Merchant))
	}
	if row.Merchant == "" {
		row.Merchant = firstRunes(descCell, 60)
	}
	return row, nil
}

func orZero(s string) string {
	if strings.TrimSpace(s) == "" {
		return "0"
	}
	return s
}

func cell(rec []string, idx int) string {
	if idx < 0 || idx >= len(rec) {
		return ""
	}
	return strings.TrimSpace(rec[idx])
}

func isBlankRow(rec []string) bool {
	for _, c := range rec {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

func firstRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func detectSeparator(data string) rune {
	first := data
	if i := strings.IndexAny(data, "\r\n"); i >= 0 {
		first = data[:i]
	}
	counts := map[rune]int{}
	inQuotes := false
	for _, r := range first {
		switch r {
		case '"':
			inQuotes = !inQuotes
		case ',', ';', '\t':
			if !inQuotes {
				counts[r]++
			}
		}
	}
	best, bestN := ',', counts[',']
	for sep, n := range counts {
		if n > bestN {
			best, bestN = sep, n
		}
	}
	if bestN == 0 {
		if strings.Contains(first, ";") {
			return ';'
		}
		if strings.Contains(first, "\t") {
			return '\t'
		}
	}
	return best
}

// String renders mapping for logs/debug.
func (m *ColumnMapping) String() string {
	return fmt.Sprintf("date=%d desc=%d amount=%d debit=%d credit=%d merchant=%d",
		m.Date, m.Description, m.Amount, m.Debit, m.Credit, m.Merchant)
}

var _ = strconv.Itoa // keep strconv import if unused after refactors
