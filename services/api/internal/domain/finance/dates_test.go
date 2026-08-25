package finance

import "testing"

func TestSniffDateLayout(t *testing.T) {
	cases := []struct {
		name   string
		values []string
		want   string
	}{
		{"iso", []string{"2026-08-01", "2026-08-02"}, "2006-01-02"},
		{"dmy slash", []string{"01/08/2026", "15/09/2026"}, "02/01/2006"},
		{"mdy impossible", []string{"08/23/2026", "09/14/2026"}, "01/02/2006"},
		{"dmy dash", []string{"01-08-2026", "31-12-2025"}, "02-01-2006"},
		{"text month", []string{"01 Aug 2026", "15 Sep 2026"}, "02 Jan 2006"},
		{"skips empties", []string{"", "2026-08-01", ""}, "2006-01-02"},
		{"unknown", []string{"not a date"}, ""},
	}
	for _, c := range cases {
		if got := SniffDateLayout(c.values); got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}

func TestParseDate(t *testing.T) {
	got, err := ParseDate(" 01/08/2026 ", "")
	if err != nil || got != "2026-08-01" {
		t.Fatalf("got %q err %v", got, err)
	}
	got, err = ParseDate("08/01/2026", "01/02/2006")
	if err != nil || got != "2026-08-01" {
		t.Fatalf("override format: got %q err %v", got, err)
	}
	if _, err := ParseDate("garbage", ""); err == nil {
		t.Fatal("expected error for garbage date")
	}
	if _, err := ParseDate("", ""); err == nil {
		t.Fatal("expected error for empty date")
	}
}
