package bot

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"xray-status/internal/checker"
	"xray-status/internal/config"
	"xray-status/internal/store"
)

func ctlBot(t *testing.T, caps ...string) *Bot {
	t.Helper()
	st, err := store.Open("sqlite", filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	_ = st.EnableSecrets("00000000000000000000000000000000000000000000000000000000000000dd")
	return &Bot{st: st, cfg: config.Config{ControlCaps: caps, InternalPort: "8081", TZ: "UTC"},
		admins: map[int64]bool{1: true}}
}

func TestSubGating(t *testing.T) {
	// По умолчанию (CONTROL_CAPS не задан) управление подпиской ВКЛЮЧЕНО.
	tb := ctlBot(t)
	if out := cmdSub(tb, nil, 1); strings.Contains(out, "выключено") {
		t.Fatalf("sub should be enabled by default: %s", out)
	}
	// Если CONTROL_CAPS задан явно и без subscription — выключено.
	tb2 := ctlBot(t, "maintenance")
	if out := cmdSub(tb2, nil, 1); !strings.Contains(out, "выключено") {
		t.Fatalf("sub should be gated when caps set without subscription: %s", out)
	}
}

func TestServerHideShowAndAudit(t *testing.T) {
	tb := ctlBot(t)
	now := time.Now().UTC()
	_, _ = tb.st.PollWrite([]checker.Proxy{
		{StableID: "a", Name: "DE Frankfurt", Online: true, LatencyMs: 50},
	}, store.PollWriteParams{Now: now.Unix(), Today: now.Format("2006-01-02"),
		PollInterval: 60, CutoffDay: "2000-01-01", SampleRetainDays: 31})

	out := cmdServerToggle(tb, []string{"Frankfurt", "off"}, 1)
	if !strings.Contains(out, "Скрыто") {
		t.Fatalf("hide: %s", out)
	}
	hid, _ := tb.st.HiddenSet()
	if !hid["DE Frankfurt"] {
		t.Fatalf("DE Frankfurt should be hidden by display-name match: %v", hid)
	}
	// показать обратно
	if out := cmdServerToggle(tb, []string{"Frankfurt", "on"}, 1); !strings.Contains(out, "странице") {
		t.Fatalf("show: %s", out)
	}
	hid2, _ := tb.st.HiddenSet()
	if hid2["DE Frankfurt"] {
		t.Fatalf("DE Frankfurt should be visible again")
	}
	if a := cmdAudit(tb); !strings.Contains(a, "server_hide") {
		t.Fatalf("audit should record action: %s", a)
	}
}

func TestSubURLSet(t *testing.T) {
	tb := ctlBot(t, "subscription")
	out := cmdSub(tb, []string{"url", "https://p/sub?t=1"}, 1)
	if !strings.Contains(out, "сохранена") {
		t.Fatalf("sub url set: %s", out)
	}
	if !tb.st.HasSubscriptionURL() {
		t.Fatal("sub url not persisted")
	}
}
