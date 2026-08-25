package knowledge

import "testing"

func TestNormalizeURL(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "https://Example.com", want: "https://example.com"},
		{in: "https://example.com/", want: "https://example.com"},
		{in: "http://example.com:80/a", want: "http://example.com/a"},
		{in: "https://example.com:443/a", want: "https://example.com/a"},
		{in: "HTTPS://EXAMPLE.COM/Path", want: "https://example.com/Path"}, // path case preserved
		{in: "https://example.com?utm_source=x&id=2", want: "https://example.com?id=2"},
		{in: "https://example.com/p?fbclid=abc&utm_campaign=z&q=1", want: "https://example.com/p?q=1"},
		{in: "  https://example.com/page#section  ", want: "https://example.com/page"},
		{in: "https://example.com/blog/", want: "https://example.com/blog"},
		{in: "ftp://example.com", wantErr: true},
		{in: "not a url", wantErr: true},
		{in: "", wantErr: true},
		{in: "https:///nohost", wantErr: true},
	}
	for _, tc := range cases {
		got, err := NormalizeURL(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("NormalizeURL(%q) = %q, want error", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("NormalizeURL(%q): unexpected error %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("NormalizeURL(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// Same page via different campaign URLs must normalize identically — the
// dedupe guarantee behind bookmark idempotence.
func TestNormalizeURLCampaignEquivalence(t *testing.T) {
	a, err := NormalizeURL("https://blog.dev/posts/go?utm_source=hackernews&utm_medium=social")
	if err != nil {
		t.Fatal(err)
	}
	b, err := NormalizeURL("https://BLOG.dev/posts/go/?utm_source=twitter&fbclid=xyz")
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("campaign variants diverged: %q vs %q", a, b)
	}
}

func TestSanitizeFTSQuery(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{in: "personal OS", want: `"personal" "os"`},
		{in: "Hello, World!", want: `"hello" "world"`},
		{in: `quote" AND (stuff) OR NOT`, want: `"quote" "and" "stuff" "or" "not"`}, // operators neutralized as terms
		{in: "c++ tips", want: `"c" "tips"`},
		{in: "café résumé", want: `"café" "résumé"`}, // unicode preserved
		{in: "  ... !!! --- ", want: ""},
		{in: "", want: ""},
	}
	for _, tc := range cases {
		if got := SanitizeFTSQuery(tc.in); got != tc.want {
			t.Errorf("SanitizeFTSQuery(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
