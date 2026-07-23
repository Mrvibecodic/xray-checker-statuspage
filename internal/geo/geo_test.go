package geo

import "testing"

func TestDetectCountry(t *testing.T) {
	cases := map[string]string{
		"🇳🇱 NL-Amsterdam": "nl",
		"DE Frankfurt":    "de",
		"US New York":     "us",
		"Финляндия-1":     "fi",
		"random-host":     "",
		"Europe":          "eu",
		"EU Frankfurt":    "de",
		"🇯🇵 Tokyo":        "jp",
		"🇬🇧 London":       "gb",
		"🇧🇷 SP":           "br",
		"🇹🇭 BKK":          "th",
		"🇻🇳 Hanoi":        "vn",
	}
	for in, want := range cases {
		if got := DetectCountry(in); got != want {
			t.Errorf("DetectCountry(%q)=%q want %q", in, got, want)
		}
	}
}

func TestDisplayName(t *testing.T) {
	cases := []struct{ name, cc, want string }{
		{"🇳🇱 NL-Amsterdam", "nl", "Amsterdam"},
		{"DE Frankfurt", "de", "Frankfurt"},
		{"USA New York", "us", "New York"},
		{"PlainHost", "", "PlainHost"},
		{"", "ru", ""},
		// Префикс — часть имени, а не код определённой страны: не срезать.
		{"🇩🇪 IN (Xray-Checker)", "de", "IN (Xray-Checker)"},
		{"🇫🇮 NH (Xray-Checker)", "fi", "NH (Xray-Checker)"},
		// Префикс совпадает со страной флага — дубль, срезаем.
		{"🇮🇳 IN Mumbai", "in", "Mumbai"},
	}
	for _, c := range cases {
		if got := DisplayName(c.name, c.cc); got != c.want {
			t.Errorf("DisplayName(%q,%q)=%q want %q", c.name, c.cc, got, c.want)
		}
	}
}
