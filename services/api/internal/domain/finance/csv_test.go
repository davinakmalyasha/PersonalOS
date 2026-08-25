package finance

import (
	"strings"
	"testing"
)

const sampleEN = `Date,Description,Amount
2026-08-01,STARBUCKS COFFEE,-125.00
2026-08-02,SALARY AUGUST,5000.00
2026-08-03,UNKNOWN SHOP,10.00
`

const sampleID = `Tanggal;Keterangan;Debit;Kredit
01/08/2026;TRANSFER KE TOKO ABC;250000;;
02/08/2026;GAJIAN BULANAN;;8500000;
03/08/2026;TOP UP E-WALLET;100000;;
bad-not-a-date;BROKEN ROW;x;;
`

const sampleNoHeader = `col1,col2,col3
a,b,c
`

func TestParseCSVEnglishSingleAmount(t *testing.T) {
	rows, errs, err := ParseCSV(strings.NewReader(sampleEN), nil, "")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("expected no row errors, got %v", errs)
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	if rows[0].Date != "2026-08-01" || rows[0].Amount != -12500 || rows[0].RawDesc != "STARBUCKS COFFEE" {
		t.Fatalf("row0 = %+v", rows[0])
	}
	if rows[1].Amount != 500000 {
		t.Fatalf("row1 amount = %d", rows[1].Amount)
	}
	if rows[0].Merchant == "" {
		t.Fatal("merchant should default from description")
	}
}

func TestParseCSVIndonesianSplitColumns(t *testing.T) {
	rows, _, err := ParseCSV(strings.NewReader(sampleID), nil, "")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 valid rows (broken skipped), got %d", len(rows))
	}
	// Debit 250000 → -25000000 minor units.
	if rows[0].Amount != -25000000 {
		t.Fatalf("debit sign/amount wrong: %d", rows[0].Amount)
	}
	// Credit 8.500.000 → +850000000.
	if rows[1].Amount != 850000000 {
		t.Fatalf("credit amount wrong: %d", rows[1].Amount)
	}
	if rows[2].Amount != -10000000 || rows[2].Date != "2026-08-03" {
		t.Fatalf("row2 wrong: %+v", rows[2])
	}
}

func TestParseCSVExplicitOverride(t *testing.T) {
	override := &ColumnMapping{Date: 0, Description: 1, Amount: 2, Merchant: -1, Debit: -1, Credit: -1}
	rows, _, err := ParseCSV(strings.NewReader(sampleEN), override, "2006-01-02")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(rows) != 3 || rows[0].Date != "2026-08-01" {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestParseCSVErrorsCollected(t *testing.T) {
	_, errs, err := ParseCSV(strings.NewReader(sampleID), nil, "")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(errs) != 1 || errs[0].Line != 5 {
		t.Fatalf("expected one error at line 5, got %+v", errs)
	}
}

func TestDetectMappingFailure(t *testing.T) {
	if _, _, err := ParseCSV(strings.NewReader(sampleNoHeader), nil, ""); err == nil {
		t.Fatal("expected mapping failure for unrecognized header")
	}
}
