package poller

import (
	"testing"

	"xray-status/internal/checker"
	"xray-status/internal/config"
	"xray-status/internal/store"
	"xray-status/internal/storetest"
)

func TestDetectPingEvents(t *testing.T) {
	st, err := store.Open(storetest.DSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	p := New(config.Config{TZ: "UTC", PollInterval: 60}, st, nil)
	maint := map[string]bool{}

	px := []checker.Proxy{
		{StableID: "a", Name: "A", Online: true, LatencyMs: 150},
		{StableID: "b", Name: "B", Online: true, LatencyMs: 50},
	}
	ev := p.detectPingEvents(px, 100, maint)
	if len(ev) != 1 || ev[0].Type != EventHighPing || ev[0].Name != "A" || ev[0].Latency != 150 {
		t.Fatalf("expected one high-ping for A, got %+v", ev)
	}
	// повтор — без событий (анти-флап)
	if ev2 := p.detectPingEvents(px, 100, maint); len(ev2) != 0 {
		t.Fatalf("repeat should be silent, got %+v", ev2)
	}
	// A восстановился
	px2 := []checker.Proxy{{StableID: "a", Name: "A", Online: true, LatencyMs: 40}}
	ev3 := p.detectPingEvents(px2, 100, maint)
	if len(ev3) != 1 || ev3[0].Type != EventPingOK {
		t.Fatalf("expected ping_ok, got %+v", ev3)
	}
	// порог 0 — детектор выключен
	if ev4 := p.detectPingEvents(px, 0, maint); len(ev4) != 0 {
		t.Fatalf("threshold 0 must disable, got %+v", ev4)
	}
}
