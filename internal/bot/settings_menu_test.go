package bot

import (
	"strings"
	"testing"

	"xray-status/internal/config"
	"xray-status/internal/store"
	"xray-status/internal/storetest"
)

func setBot(t *testing.T) *Bot {
	t.Helper()
	st, err := store.Open(storetest.DSN(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return &Bot{st: st, cfg: config.Config{TZ: "UTC"}, admins: map[int64]bool{1: true}}
}

func TestSettingCallbacks(t *testing.T) {
	tb := setBot(t)

	// alert toggle (default on -> off)
	tb.handleSettingCallback(1, 10, "set:alert")
	if tb.st.AlertOnDown() {
		t.Fatal("alert should be off after toggle")
	}
	// ping preset
	tb.handleSettingCallback(1, 10, "set:ping:300")
	if tb.st.PingThreshold() != 300 {
		t.Fatalf("ping=%d", tb.st.PingThreshold())
	}
	tb.handleSettingCallback(1, 10, "set:ping:0")
	if tb.st.PingThreshold() != 0 {
		t.Fatal("ping should be off")
	}
	// summary preset
	tb.handleSettingCallback(1, 10, "set:sum:09:00")
	if tb.st.DailySummaryTime() != "09:00" {
		t.Fatalf("sum=%q", tb.st.DailySummaryTime())
	}
	tb.handleSettingCallback(1, 10, "set:sum:off")
	if tb.st.DailySummaryTime() != "" {
		t.Fatal("sum should be off")
	}
	// domain -> awaiting input
	txt, _ := tb.handleSettingCallback(1, 42, "set:domain")
	if !strings.Contains(txt, "домен") {
		t.Fatalf("domain prompt: %s", txt)
	}
	if tb.st.GetBotState(1, "await") != "domain" || tb.st.GetBotState(1, "await_msg") != "42" {
		t.Fatal("await state not set")
	}
	// home clears await + renders settings
	st, kb := tb.handleSettingCallback(1, 10, "set:home")
	if !strings.Contains(st, "Настройки") || kb == nil {
		t.Fatal("home should render settings")
	}
	if tb.st.GetBotState(1, "await") != "" {
		t.Fatal("await should be cleared")
	}
}
