package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"xray-status/internal/config"
	"xray-status/internal/store"
)

func TestInternalSubFilters(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "vless://a@h:443#NL\nvless://b@h:443#DE")
	}))
	defer upstream.Close()

	st, err := store.Open("sqlite", filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	_ = st.EnableSecrets("00000000000000000000000000000000000000000000000000000000000000cc")
	_ = st.SetSubscriptionURL(upstream.URL)
	_ = st.SetServerEnabled("DE", false)

	in := NewInternal(config.Config{}, st)
	rr := httptest.NewRecorder()
	in.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/sub", nil))
	if rr.Code != 200 {
		t.Fatalf("code %d", rr.Code)
	}
	if body := rr.Body.String(); body != "vless://a@h:443#NL" {
		t.Fatalf("filtered sub wrong: %q", body)
	}

	// без подписки — 404
	st2, _ := store.Open("sqlite", filepath.Join(t.TempDir(), "t2.db"))
	defer st2.Close()
	rr2 := httptest.NewRecorder()
	NewInternal(config.Config{}, st2).Handler().ServeHTTP(rr2, httptest.NewRequest(http.MethodGet, "/sub", nil))
	if rr2.Code != 404 {
		t.Fatalf("expected 404 without sub, got %d", rr2.Code)
	}
}
