package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"xray-status/internal/config"
	"xray-status/internal/store"
	"xray-status/internal/storetest"
)

func testStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(storetest.DSN(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func getPage(t *testing.T, cfg config.Config, theme, path string) (int, string, http.Header) {
	t.Helper()
	st := testStore(t)
	if theme != "" {
		st.SetSetting("theme", theme)
	}
	h := New(cfg, st).Handler()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
	return rr.Code, rr.Body.String(), rr.Header()
}

// Английская локаль: lang="en", английские строки, ни одного незаменённого
// __T_*__ — для всех трёх тем.
func TestEnglishPage(t *testing.T) {
	cfg := config.Config{Title: "T", Subtitle: "S", Days: 30, TZ: "UTC", PollInterval: 300, Locale: "en"}
	for theme, want := range map[string]string{
		"":        "Servers online",
		"v2":      "Active servers",
		"minimal": "Active servers",
		"light":   "Servers online",
		"claude":  "Servers online",
	} {
		code, page, _ := getPage(t, cfg, theme, "/")
		if code != 200 {
			t.Fatalf("theme %q: index code %d", theme, code)
		}
		for _, s := range []string{`lang="en"`, want, "Loading…", "Average ping", "uptime: ", "status: "} {
			if theme == "minimal" && s == "Average ping" {
				continue // minimal-тема без статистики пинга
			}
			if !strings.Contains(page, s) {
				t.Errorf("theme %q: page missing %q", theme, s)
			}
		}
		for _, s := range []string{`lang="ru"`, "Серверов онлайн", "Загрузка", "__T_"} {
			if strings.Contains(page, s) {
				t.Errorf("theme %q: en page contains %q", theme, s)
			}
		}
	}
}

// Русская локаль по умолчанию: lang="ru", русские строки, ни одного __T_*__.
func TestRussianPageDefault(t *testing.T) {
	cfg := config.Config{Title: "T", Subtitle: "S", Days: 30, TZ: "UTC", PollInterval: 300}
	for _, theme := range []string{"", "v2", "minimal"} {
		code, page, _ := getPage(t, cfg, theme, "/")
		if code != 200 {
			t.Fatalf("theme %q: index code %d", theme, code)
		}
		if !strings.Contains(page, `lang="ru"`) {
			t.Errorf("theme %q: missing lang=ru", theme)
		}
		if !strings.Contains(page, "Загрузка…") {
			t.Errorf("theme %q: missing RU loading", theme)
		}
		if strings.Contains(page, "__T_") {
			t.Errorf("theme %q: unreplaced placeholder", theme)
		}
	}
}

// Без пароля — всё открыто, как раньше.
func TestNoAuthByDefault(t *testing.T) {
	cfg := config.Config{Title: "T", Subtitle: "S", Days: 30, TZ: "UTC", PollInterval: 300}
	for _, path := range []string{"/", "/api/summary", "/health", "/healthz"} {
		if code, _, _ := getPage(t, cfg, "", path); code != 200 {
			t.Errorf("path %s: code %d without password", path, code)
		}
	}
}

// С паролем: без кредов — 401 + WWW-Authenticate, с верным — 200,
// /health и /healthz открыты. API тоже под замком.
func TestBasicAuth(t *testing.T) {
	cfg := config.Config{Title: "T", Subtitle: "S", Days: 30, TZ: "UTC", PollInterval: 300, WebPassword: "secret"}
	st := testStore(t)
	h := New(cfg, st).Handler()

	serve := func(path, user, pass string, withAuth bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if withAuth {
			req.SetBasicAuth(user, pass)
		}
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr
	}

	for _, path := range []string{"/", "/api/summary"} {
		rr := serve(path, "", "", false)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("path %s: code %d without creds, want 401", path, rr.Code)
		}
		if rr.Header().Get("WWW-Authenticate") == "" {
			t.Errorf("path %s: missing WWW-Authenticate", path)
		}
		if rr := serve(path, "anyone", "wrong", true); rr.Code != http.StatusUnauthorized {
			t.Errorf("path %s: code %d with wrong password, want 401", path, rr.Code)
		}
		if rr := serve(path, "anyone", "secret", true); rr.Code != 200 {
			t.Errorf("path %s: code %d with correct password, want 200", path, rr.Code)
		}
	}

	for _, path := range []string{"/health", "/healthz"} {
		if rr := serve(path, "", "", false); rr.Code != 200 {
			t.Errorf("path %s: code %d without creds, want 200 (probes stay open)", path, rr.Code)
		}
	}
}

// С заданным WEB_USER логин тоже проверяется.
func TestBasicAuthWithUser(t *testing.T) {
	cfg := config.Config{Title: "T", Subtitle: "S", Days: 30, TZ: "UTC", PollInterval: 300, WebPassword: "secret", WebUser: "admin"}
	h := New(cfg, testStore(t)).Handler()

	serve := func(user, pass string) int {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.SetBasicAuth(user, pass)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr.Code
	}

	if code := serve("admin", "secret"); code != 200 {
		t.Errorf("correct user+password: code %d, want 200", code)
	}
	if code := serve("intruder", "secret"); code != http.StatusUnauthorized {
		t.Errorf("wrong user: code %d, want 401", code)
	}
	if code := serve("admin", "wrong"); code != http.StatusUnauthorized {
		t.Errorf("wrong password: code %d, want 401", code)
	}
}
