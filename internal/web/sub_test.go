package web

import (
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"xray-status/internal/config"
	"xray-status/internal/store"
	"xray-status/internal/sub"
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
	dec, err := base64.StdEncoding.DecodeString(rr.Body.String())
	if err != nil {
		t.Fatalf("output not base64: %v (%q)", err, rr.Body.String())
	}
	if string(dec) != "vless://a@h:443#NL" {
		t.Fatalf("filtered sub wrong: %q", dec)
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

func TestInternalSubMergesMultipleAndPerServerDisable(t *testing.T) {
	upA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "vless://a@h1:443#NL\nvless://x@h9:443#DE")
	}))
	defer upA.Close()
	upB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "vless://b@h2:443#NL")
	}))
	defer upB.Close()

	st, err := store.Open("sqlite", filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	_ = st.EnableSecrets("00000000000000000000000000000000000000000000000000000000000000dd")
	if _, err := st.AddSubscriptionURLs([]string{upA.URL, upB.URL}); err != nil {
		t.Fatal(err)
	}

	in := NewInternal(config.Config{}, st)
	fetch := func() []string {
		rr := httptest.NewRecorder()
		in.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/sub", nil))
		if rr.Code != 200 {
			t.Fatalf("code %d", rr.Code)
		}
		dec, err := base64.StdEncoding.DecodeString(rr.Body.String())
		if err != nil {
			t.Fatalf("not base64: %v", err)
		}
		var out []string
		for _, l := range strings.Split(string(dec), "\n") {
			if strings.TrimSpace(l) != "" {
				out = append(out, l)
			}
		}
		return out
	}

	lines := fetch()
	if len(lines) != 3 {
		t.Fatalf("merged want 3 lines, got %d: %v", len(lines), lines)
	}
	// дубль #NL разведён тегом, #DE без тега
	var nlTagged []string
	deOK := false
	for _, l := range lines {
		r := sub.Remark(l)
		switch {
		case r == "DE":
			deOK = true
		case r == "NL":
			t.Fatalf("duplicate NL must be tagged: %q", l)
		case strings.HasPrefix(r, "NL ") && sub.StripTag(r) == "NL":
			nlTagged = append(nlTagged, r)
		}
	}
	if !deOK || len(nlTagged) != 2 {
		t.Fatalf("want DE + 2 tagged NL; deOK=%v nlTagged=%v", deOK, nlTagged)
	}

	// Выключаем ОДИН тегированный NL — должен исчезнуть только он.
	if err := st.SetServerEnabled(nlTagged[0], false); err != nil {
		t.Fatal(err)
	}
	lines2 := fetch()
	if len(lines2) != 2 {
		t.Fatalf("after per-server disable want 2 lines, got %d: %v", len(lines2), lines2)
	}
	for _, l := range lines2 {
		if sub.Remark(l) == nlTagged[0] {
			t.Fatalf("disabled server %q still present", nlTagged[0])
		}
	}
}
