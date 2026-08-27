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

// Ссылки на статику внутри страницы должны быть относительными: иначе за
// reverse proxy с префиксом пути (example.com/<prefix>/) браузер уходит за
// флагами и фавиконом в корень домена. См. issue #16.
func TestPageStaticRefsAreRelative(t *testing.T) {
	st, err := store.Open(storetest.DSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.SetAsset("favicon", "image/png", []byte("fake")); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{Title: "T", Subtitle: "S", Days: 30, TZ: "UTC", PollInterval: 60}
	h := New(cfg, st).Handler()

	for _, theme := range []string{"light", "dark", "claude", "claude-dark", "v2", "minimal"} {
		if err := st.SetSetting("theme", theme); err != nil {
			t.Fatal(err)
		}
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
		if rr.Code != 200 {
			t.Fatalf("тема %s: код %d", theme, rr.Code)
		}
		page := rr.Body.String()
		for _, bad := range []string{`src="/flags/`, `href="/favicon`, `src="/favicon`, `src="/logo`} {
			if strings.Contains(page, bad) {
				t.Errorf("тема %s: абсолютная ссылка %s ломает раздачу под префиксом пути", theme, bad)
			}
		}
		if !strings.Contains(page, `src="flags/`) {
			t.Errorf("тема %s: относительных ссылок на флаги нет", theme)
		}
		if !strings.Contains(page, `href="favicon.ico`) {
			t.Errorf("тема %s: относительной ссылки на фавикон нет", theme)
		}
	}
}
