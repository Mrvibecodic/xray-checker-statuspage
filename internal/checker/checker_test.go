package checker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Fetch должен понимать оба формата ответа чекера: конверт {"data":[...]} и
// голый массив; поля groupName/lastCheck (чекер ≥1.3.0) — доезжать до Proxy.
func TestFetchEnvelope(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/public/proxies" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"data":[
		  {"stableId":"a","name":"Alpha","groupName":"","online":true,"latencyMs":42,"lastCheck":1700000000},
		  {"stableId":"b","name":"Grp | proxy","groupName":"Grp","online":false,"latencyMs":0,"lastCheck":0}
		]}`))
	}))
	defer ts.Close()

	ps, err := New(ts.URL).Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 2 {
		t.Fatalf("ожидалось 2 прокси, got %d", len(ps))
	}
	if ps[0].StableID != "a" || !ps[0].Online || ps[0].LatencyMs != 42 || ps[0].LastCheck != 1700000000 {
		t.Fatalf("proxy[0]: %+v", ps[0])
	}
	if ps[1].GroupName != "Grp" || ps[1].Online {
		t.Fatalf("proxy[1]: %+v", ps[1])
	}
}

func TestFetchBareArray(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"stableId":"x","name":"X","online":true,"latencyMs":1}]`))
	}))
	defer ts.Close()

	ps, err := New(ts.URL).Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 1 || ps[0].StableID != "x" {
		t.Fatalf("голый массив: %+v", ps)
	}
}

func TestFetchConfig(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/config" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"success":true,"data":{"checkInterval":120}}`))
	}))
	defer ts.Close()

	cfg, err := New(ts.URL).FetchConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CheckInterval != 120 {
		t.Fatalf("checkInterval: %+v", cfg)
	}
}
