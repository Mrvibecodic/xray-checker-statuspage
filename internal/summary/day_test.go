package summary

import "testing"

func TestDayLabelLocale(t *testing.T) {
	if got := dayLabel("2026-10-05", "ru"); got != "05 окт" {
		t.Errorf("ru: got %q", got)
	}
	if got := dayLabel("2026-10-05", "en"); got != "05 Oct" {
		t.Errorf("en: got %q", got)
	}
	if got := dayLabel("junk", "en"); got != "junk" {
		t.Errorf("invalid date passthrough: got %q", got)
	}
}
