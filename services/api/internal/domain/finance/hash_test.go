package finance

import "testing"

func TestNormalizeDescription(t *testing.T) {
	cases := []struct{ in, want string }{
		{"STARBUCKS COFFEE #123", "starbucks coffee 123"},
		{"  TRX   PAYMENT--IDR100.000!! ", "trx payment idr100 000"},
		{"Pembayaran QRIS-Merchant", "pembayaran qris merchant"},
		{"", ""},
		{"!!!", ""},
		{"ALPHA   beta", "alpha beta"},
	}
	for _, c := range cases {
		if got := NormalizeDescription(c.in); got != c.want {
			t.Errorf("Normalize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDescriptionHashStableAndDistinct(t *testing.T) {
	a1 := DescriptionHash("Starbucks Coffee #123")
	a2 := DescriptionHash("starbucks   coffee #123!")
	if a1 != a2 {
		t.Fatalf("expected same hash for equivalent descriptions")
	}
	b := DescriptionHash("Different Merchant")
	if a1 == b {
		t.Fatalf("expected distinct hashes")
	}
	if len(a1) != 64 {
		t.Fatalf("expected sha256 hex length 64, got %d", len(a1))
	}
}

func TestDedupeKeyFormat(t *testing.T) {
	h := DescriptionHash("x")
	want := "2026-08-01|-125000|" + h
	got := DedupeKey("2026-08-01", -125000, h)
	if got != want {
		t.Fatalf("key = %q, want %q", got, want)
	}
}
