package bot

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"xray-status/internal/config"
	"xray-status/internal/store"
)

func newDiagBot(t *testing.T, subURL, checkerURL string) *Bot {
	t.Helper()
	st, err := store.Open("sqlite", filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return &Bot{
		st:     st,
		cfg:    config.Config{SubscriptionURL: subURL, CheckerURL: checkerURL, InternalPort: "8081"},
		admins: map[int64]bool{1: true},
	}
}

func TestSubDiagText_chainOK(t *testing.T) {
	subTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "Happ/1.0" {
			t.Errorf("диагностика должна ходить под Happ/1.0, got %q", r.Header.Get("User-Agent"))
		}
		if r.Header.Get("X-Hwid") == "" {
			t.Error("диагностика должна слать X-Hwid")
		}
		_, _ = w.Write([]byte(`[
		  {"remarks":"Bal","outbounds":[{"tag":"proxy","protocol":"vless"},{"tag":"proxy-2","protocol":"vless"},{"tag":"direct","protocol":"freedom"}]},
		  {"remarks":"Solo","outbounds":[{"tag":"proxy","protocol":"vless"}]}
		]`))
	}))
	defer subTS.Close()
	chkTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[
		  {"stableId":"a","name":"Bal | proxy","groupName":"Bal","online":true,"latencyMs":10},
		  {"stableId":"b","name":"Bal | proxy-2","groupName":"Bal","online":false,"latencyMs":0},
		  {"stableId":"c","name":"Solo","groupName":"","online":true,"latencyMs":20}
		]}`))
	}))
	defer chkTS.Close()

	tb := newDiagBot(t, subTS.URL, chkTS.URL)
	s := tb.subDiagText(context.Background())
	if !strings.Contains(s, "✅") {
		t.Fatalf("ожидался ✅-вердикт, got:\n%s", s)
	}
	if !strings.Contains(s, "конфигов: 2") || !strings.Contains(s, "«Bal» (2 узл.)") {
		t.Fatalf("нет разбора подписки:\n%s", s)
	}
	if !strings.Contains(s, "прокси: 3, онлайн: 2") {
		t.Fatalf("нет сводки чекера:\n%s", s)
	}
}

func TestSubDiagText_checkerBlindToGroups(t *testing.T) {
	subTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"remarks":"Bal","outbounds":[{"tag":"proxy","protocol":"vless"},{"tag":"proxy-2","protocol":"vless"}]}]`))
	}))
	defer subTS.Close()
	chkTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"stableId":"x","name":"Bal","groupName":"","online":false,"latencyMs":0}]}`))
	}))
	defer chkTS.Close()

	tb := newDiagBot(t, subTS.URL, chkTS.URL)
	s := tb.subDiagText(context.Background())
	if !strings.Contains(s, "SUBSCRIPTION_JSON_FORMAT") {
		t.Fatalf("ожидался вердикт про json-режим чекера:\n%s", s)
	}
}

func TestSubDiagText_panelNoJSON(t *testing.T) {
	b64 := base64.StdEncoding.EncodeToString([]byte("vless://x@a:1#A\nvless://y@b:2#B"))
	subTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(b64))
	}))
	defer subTS.Close()
	chkTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer chkTS.Close()

	tb := newDiagBot(t, subTS.URL, chkTS.URL)
	s := tb.subDiagText(context.Background())
	if !strings.Contains(s, "серверов: 2") || !strings.Contains(s, "не отдала JSON") {
		t.Fatalf("ожидался вердикт про панель без JSON:\n%s", s)
	}
}
