package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"xray-status/internal/checker"
	"xray-status/internal/config"
	"xray-status/internal/store"
	"xray-status/internal/storetest"
)

func TestServeSummaryAndPage(t *testing.T) {
	st, err := store.Open(storetest.DSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Now().UTC()
	if _, err := st.PollWrite([]checker.Proxy{
		{StableID: "a1", Name: "DE Frankfurt", Online: true, LatencyMs: 50},
	}, store.PollWriteParams{Now: now.Unix(), Today: now.Format("2006-01-02"),
		PollInterval: 60, CutoffDay: "2000-01-01", SampleRetainDays: 31}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateIncident("DB slow", "major", []string{"DE Frankfurt"}, "x", 1, false); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddMaintenance("DE Frankfurt", now.Unix()-10, now.Unix()+3600, "", 1); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{Title: "T", Subtitle: "S", Days: 30, TZ: "UTC", PollInterval: 60, ServerHeader: "nginx"}
	h := New(cfg, st).Handler()

	// /api/summary должен содержать инцидент и флаг обслуживания
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/summary", nil))
	body := rr.Body.String()
	if rr.Code != 200 {
		t.Fatalf("summary code %d", rr.Code)
	}
	if !strings.Contains(body, "DB slow") {
		t.Error("summary missing incident")
	}
	if !strings.Contains(body, `"maintenance":true`) {
		t.Error("summary missing maintenance flag")
	}

	// страница должна содержать встроенный рендер инцидентов
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, httptest.NewRequest(http.MethodGet, "/", nil))
	page := rr2.Body.String()
	if !strings.Contains(page, "renderIncidents") || !strings.Contains(page, `id="incidents"`) {
		t.Error("page missing incident rendering hooks")
	}
	if rr2.Header().Get("Server") != "nginx" {
		t.Error("server header not masked")
	}
}

// TestPageRandomizedPerLoad — анти-фингерпринт: каждая загрузка отдаёт разный
// HTML (свежий префикс классов + шум), но JSON-доступы и критичные CSS не ломаются.
func TestPageRandomizedPerLoad(t *testing.T) {
	st, err := store.Open(storetest.DSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	cfg := config.Config{Title: "T", Subtitle: "S", Days: 30, TZ: "UTC", PollInterval: 300}
	h := New(cfg, st).Handler()

	get := func() string {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
		if rr.Code != 200 {
			t.Fatalf("index code %d", rr.Code)
		}
		return rr.Body.String()
	}
	a, b := get(), get()
	if a == b {
		t.Fatal("страница не рандомизируется между загрузками")
	}
	for _, page := range []string{a, b} {
		// JSON-ключи и JS-доступы не должны мангулиться
		for _, must := range []string{"data.title", "s.name", "d.label", "data.incidents", "data.stats", `style="top:`} {
			if !strings.Contains(page, must) {
				t.Errorf("сломан фрагмент после рандомизации: %q", must)
			}
		}
		// токенизированный класс не должен оставаться «сырым»
		if strings.Contains(page, `.inc-card{`) {
			t.Error("inc-card не префиксован (рандомизация не сработала)")
		}
	}
}

func TestMinimalThemeServed(t *testing.T) {
	st, err := store.Open(storetest.DSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	_ = st.SetSetting("theme", "minimal")
	cfg := config.Config{Title: "T", Subtitle: "S", Days: 30, TZ: "UTC", PollInterval: 300}
	h := New(cfg, st).Handler()
	get := func() string {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
		if rr.Code != 200 {
			t.Fatalf("minimal index code %d", rr.Code)
		}
		return rr.Body.String()
	}
	a, b := get(), get()
	if a == b {
		t.Fatal("минимал-тема не рандомизируется между загрузками")
	}
	for _, page := range []string{a, b} {
		for _, must := range []string{"s.name", "d.label", "data.incidents", "data.servers"} {
			if !strings.Contains(page, must) {
				t.Errorf("минимал: сломан фрагмент после рандомизации: %q", must)
			}
		}
		if strings.Contains(page, ".inc-card{") {
			t.Error("минимал: inc-card не префиксован (uniquify не сработал)")
		}
		if strings.Contains(page, "s-ping") {
			t.Error("минимал: остался блок пинга (s-ping)")
		}
		if strings.Contains(page, "<!--") {
			t.Error("минимал: HTML-комментарий в выдаче")
		}
		if strings.Contains(page, "/*") {
			t.Error("минимал: CSS-комментарий в выдаче")
		}
	}
}
