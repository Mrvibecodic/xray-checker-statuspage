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
	if len(servers) != 3 {
		t.Fatalf("want 3 groups, got %d", len(servers))
	}
	byName := map[string]map[string]any{}
	for _, s := range servers {
		m := s.(map[string]any)
		byName[m["name"].(string)] = m
	}
	nl := byName["Amsterdam"]
	if nl == nil {
		t.Fatal("NL group not found / display name wrong")
	}
	if nl["members"].(int) != 2 {
		t.Errorf("NL members=%v want 2", nl["members"])
	}
	if nl["cc"].(string) != "nl" {
		t.Errorf("NL cc=%v want nl", nl["cc"])
	}
	if nl["uptime30"].(float64) != 50 {
		t.Errorf("NL uptime30=%v want 50", nl["uptime30"])
	}
	if byName["New York"]["uptime30"].(float64) != 0 {
		t.Errorf("US uptime30 want 0")
	}
	tot := p["totals"].(map[string]any)
	if tot["online"].(int) != 2 || tot["total"].(int) != 3 {
		t.Errorf("totals online/total = %v/%v want 2/3", tot["online"], tot["total"])
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
