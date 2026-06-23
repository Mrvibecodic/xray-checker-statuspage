package bot

import (
	"testing"
	"time"
)

func TestDueSummary(t *testing.T) {
	now := time.Date(2026, 6, 22, 9, 30, 0, 0, time.UTC)
	if !dueSummary("09:30", now, "") {
		t.Error("should be due at matching time, not sent yet")
	}
	if dueSummary("09:30", now, "2026-06-22") {
		t.Error("should not resend same day")
	}
	if dueSummary("09:31", now, "") {
		t.Error("wrong minute should not fire")
	}
	if dueSummary("", now, "") {
		t.Error("empty time = disabled")
	}
}
