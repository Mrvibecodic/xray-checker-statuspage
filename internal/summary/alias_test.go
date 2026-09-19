package summary

import (
	"testing"
	"time"

	"xray-status/internal/checker"
	"xray-status/internal/config"
	"xray-status/internal/store"
	"xray-status/internal/storetest"
)

func TestSummaryAliasName(t *testing.T) {
	st, err := store.Open(storetest.DSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Now().UTC()
	proxies := []checker.Proxy{
		{StableID: "a", Name: "🇺🇸 США-2", Online: true, LatencyMs: 10},
		{StableID: "b", Name: "🇩🇪 DE Frankfurt", Online: true, LatencyMs: 10},
		{StableID: "c", Name: "Plain", Online: true, LatencyMs: 10},
	}
	if _, err := st.PollWrite(proxies, store.PollWriteParams{
		Now: now.Unix(), Today: now.Format("2006-01-02"), PollInterval: 60,
		CutoffDay: "2000-01-01", SampleRetainDays: 31,
	}); err != nil {
		t.Fatal(err)
	}
	_ = st.SetAlias("🇩🇪 DE Frankfurt", "Франкфурт-1")
	_ = st.SetAlias("Plain", "Германия 5")
	if _, err := st.CreateIncident("t", "minor", []string{"🇩🇪 DE Frankfurt"}, "", 1, false); err != nil {
		t.Fatal(err)
	}

	p, err := BuildSummary(st, config.Config{Days: 30, TZ: "UTC", PollInterval: 60}, false)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][2]string{}
	for _, s := range p["servers"].([]any) {
		m := s.(map[string]any)
		got[m["sid"].(string)] = [2]string{m["name"].(string), m["cc"].(string)}
	}
	want := map[string][2]string{
		"a": {"США-2", "us"},       // без алиаса: номер не остаётся голым
		"b": {"Франкфурт-1", "de"}, // алиас как есть, флаг по исходному имени
		"c": {"Германия 5", "de"},  // флаг по алиасу
	}
	for sid, w := range want {
		if got[sid] != w {
			t.Errorf("%s: got %v want %v", sid, got[sid], w)
		}
	}
	inc := p["incidents"].([]any)[0].(map[string]any)
	aff := inc["affected"].([]any)[0].(map[string]any)
	if aff["name"] != "Франкфурт-1" || aff["cc"] != "de" {
		t.Errorf("затронутые без алиаса: %v", aff)
	}
}
