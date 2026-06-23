package bot

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"xray-status/internal/checker"
	"xray-status/internal/config"
	"xray-status/internal/poller"
	"xray-status/internal/store"
)

func TestFlag(t *testing.T) {
	if flag("nl") != "🇳🇱" {
		t.Errorf("flag(nl)=%q", flag("nl"))
	}
	if flag("") != "" || flag("x") != "" {
		t.Errorf("bad cc should give empty flag")
	}
}

func TestEventText(t *testing.T) {
	if !strings.Contains(eventText(poller.Event{Type: poller.EventServerDown, Name: "NL"}), "упал") {
		t.Error("down text")
	}
	if eventText(poller.Event{Type: poller.EventGlobalOutageSuspected}) != "" {
		t.Error("suspected must be silent")
	}
}

func TestNewDisabledWithoutToken(t *testing.T) {
	b, err := New(config.Config{}, nil)
	if err != nil || b != nil {
		t.Errorf("no token must give (nil,nil), got (%v,%v)", b, err)
	}
}

func TestStatusText(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open("sqlite", filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	cfg := config.Config{Title: "Стат", Days: 30, TZ: "UTC", PollInterval: 60}
	now := time.Now().UTC()
	_, _ = st.PollWrite([]checker.Proxy{
		{StableID: "b1", Name: "DE Frankfurt", Online: true, LatencyMs: 88},
		{StableID: "c1", Name: "US NY", Online: false},
	}, store.PollWriteParams{Now: now.Unix(), Today: now.Format("2006-01-02"),
		PollInterval: 60, CutoffDay: "2000-01-01", SampleRetainDays: 31})
	txt := statusText(st, cfg)
	if !strings.Contains(txt, "Онлайн: <b>1/2</b>") {
		t.Errorf("status text wrong:\n%s", txt)
	}
	srv := serversText(st, cfg)
	if !strings.Contains(srv, "Frankfurt") || !strings.Contains(srv, "🟢") {
		t.Errorf("servers text wrong:\n%s", srv)
	}
}
