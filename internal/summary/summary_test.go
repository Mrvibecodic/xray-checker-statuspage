package summary

import (
	"path/filepath"
	"testing"
	"time"

	"xray-status/internal/checker"
	"xray-status/internal/config"
	"xray-status/internal/store"
)

func TestBuildSummaryGroupingAndUptime(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open("sqlite", filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	cfg := config.Config{Title: "T", Subtitle: "S", Days: 30, TZ: "UTC", PollInterval: 60}
	loc := time.UTC
	now := time.Now().In(loc)
	today := now.Format("2006-01-02")

	proxies := []checker.Proxy{
		{StableID: "a1", Name: "🇳🇱 NL-Amsterdam", Online: true, LatencyMs: 42},
		{StableID: "a2", Name: "🇳🇱 NL-Amsterdam", Online: false},
		{StableID: "b1", Name: "DE Frankfurt", Online: true, LatencyMs: 88},
		{StableID: "c1", Name: "US New York", Online: false},
	}
	if _, err := st.PollWrite(proxies, store.PollWriteParams{
		Now: now.Unix(), Today: today, PollInterval: 60,
		CutoffDay: "2000-01-01", SampleRetainDays: 31,
	}); err != nil {
		t.Fatal(err)
	}

	p, err := BuildSummary(st, cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	servers := p["servers"].([]any)
	// Одинаковая ремарка с разными stableId — РАЗНЫЕ серверы (как в оригинальном
	// чекере): a1 и a2 теперь две отдельные плитки, а не одна группа.
	if len(servers) != 4 {
		t.Fatalf("want 4 servers, got %d", len(servers))
	}
	bySid := map[string]map[string]any{}
	nlCount := 0
	for _, s := range servers {
		m := s.(map[string]any)
		bySid[m["sid"].(string)] = m
		if m["name"].(string) == "Amsterdam" {
			nlCount++
		}
	}
	if nlCount != 2 {
		t.Errorf("two NL tiles expected, got %d", nlCount)
	}
	a1, a2 := bySid["a1"], bySid["a2"]
	if a1 == nil || a2 == nil {
		t.Fatal("NL tiles a1/a2 not found by sid")
	}
	if a1["members"].(int) != 1 || a2["members"].(int) != 1 {
		t.Errorf("each NL tile should have 1 member: a1=%v a2=%v", a1["members"], a2["members"])
	}
	if a1["online"].(bool) != true || a2["online"].(bool) != false {
		t.Errorf("a1 online want true (got %v), a2 online want false (got %v)", a1["online"], a2["online"])
	}
	if a1["cc"].(string) != "nl" {
		t.Errorf("NL cc=%v want nl", a1["cc"])
	}
	if a1["uptime30"].(float64) != 100 {
		t.Errorf("a1 uptime30=%v want 100", a1["uptime30"])
	}
	if a2["uptime30"].(float64) != 0 {
		t.Errorf("a2 uptime30=%v want 0", a2["uptime30"])
	}
	tot := p["totals"].(map[string]any)
	if tot["online"].(int) != 2 || tot["total"].(int) != 4 {
		t.Errorf("totals online/total = %v/%v want 2/4", tot["online"], tot["total"])
	}
	if tot["avgLatency"].(int) != 65 {
		t.Errorf("avgLatency=%v want 65", tot["avgLatency"])
	}
}

func TestSummaryMaintenanceAndIncidents(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open("sqlite", filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	cfg := config.Config{Title: "T", Days: 30, TZ: "UTC", PollInterval: 60}
	now := time.Now().UTC()
	_, _ = st.PollWrite([]checker.Proxy{
		{StableID: "a1", Name: "DE Frankfurt", Online: true, LatencyMs: 88},
		{StableID: "b1", Name: "US NY", Online: true, LatencyMs: 50},
	}, store.PollWriteParams{Now: now.Unix(), Today: now.Format("2006-01-02"),
		PollInterval: 60, CutoffDay: "2000-01-01", SampleRetainDays: 31})

	// Frankfurt в обслуживании → флаг + исключён из totals.
	if _, err := st.AddMaintenance("DE Frankfurt", now.Unix()-10, now.Unix()+3600, "upd", 1); err != nil {
		t.Fatal(err)
	}
	// активный инцидент
	if _, err := st.CreateIncident("Packet loss", "minor", []string{"US NY"}, "looking", 1, false); err != nil {
		t.Fatal(err)
	}

	p, err := BuildSummary(st, cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	var fr map[string]any
	for _, s := range p["servers"].([]any) {
		m := s.(map[string]any)
		if m["name"] == "Frankfurt" {
			fr = m
		}
	}
	if fr == nil || fr["maintenance"].(bool) != true {
		t.Fatalf("Frankfurt maintenance flag missing: %+v", fr)
	}
	tot := p["totals"].(map[string]any)
	if tot["total"].(int) != 1 { // только US NY, Frankfurt исключён
		t.Fatalf("maintenance server should be excluded from totals, total=%v", tot["total"])
	}
	inc := p["incidents"].([]any)
	if len(inc) != 1 || inc[0].(map[string]any)["title"] != "Packet loss" {
		t.Fatalf("incident not in payload: %+v", inc)
	}
}
