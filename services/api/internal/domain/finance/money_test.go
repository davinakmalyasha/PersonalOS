package finance

import "testing"

func TestParseAmount(t *testing.T) {
	cases := []struct {
		in   string
		want int64
		err  bool
	}{
		{"1,234.56", 123456, false},
		{"1.234,56", 123456, false},
		{"Rp1.234.567", 123456700, false},
		{"IDR 1,234,567.89", 123456789, false},
		{"-125.000", -12500000, false},
		{"(50.00)", -5000, false},
		{"$ (1,000)", -100000, false},
		{"0", 0, false},
		{"42", 4200, false},
		{"12,34", 1234, false},
		{"12.34", 1234, false},
		{"1.234", 123400, false},
		{"1,234", 123400, false},
		{"1,2345", 123, false},
		{"", 0, true},
		{"abc", 0, true},
		{"---", 0, true},
	}
	for _, c := range cases {
		got, err := ParseAmount(c.in)
		if c.err {
			if err == nil {
				t.Errorf("ParseAmount(%q): expected error", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseAmount(%q): unexpected error %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseAmount(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestParseAmountIndonesianStatement(t *testing.T) {
	if got, _ := ParseAmount("2.500.000,00"); got != 250000000 {
		t.Fatalf("got %d", got)
	}
	if got, _ := ParseAmount("-150.000"); got != -15000000 {
		t.Fatalf("got %d", got)
	}
}
