package web

import (
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"xray-status/internal/config"
	"xray-status/internal/store"
	"xray-status/internal/storetest"
	"xray-status/internal/sub"
)

func TestInternalSubServes(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "vless://a@h:443#NL\nvless://b@h:443#DE")
	}))
	defer upstream.Close()

	st, err := store.Open(storetest.DSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	_ = st.EnableSecrets("00000000000000000000000000000000000000000000000000000000000000cc")
	_ = st.SetSubscriptionURL(upstream.URL)

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
	got := string(dec)
	if !strings.Contains(got, "#NL") || !strings.Contains(got, "#DE") {
		t.Fatalf("both servers should be served: %q", got)
	}

	// без подписки — 404
	st2, _ := store.Open(storetest.DSN(t))
	defer st2.Close()
	rr2 := httptest.NewRecorder()
	NewInternal(config.Config{}, st2).Handler().ServeHTTP(rr2, httptest.NewRequest(http.MethodGet, "/sub", nil))
	if rr2.Code != 404 {
		t.Fatalf("expected 404 without sub, got %d", rr2.Code)
	}
}

func TestInternalSubMergesMultiple(t *testing.T) {
	upA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "vless://a@h1:443#NL\nvless://x@h9:443#DE")
	}))
	defer upA.Close()
	upB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "vless://b@h2:443#NL")
	}))
	defer upB.Close()

	st, err := store.Open(storetest.DSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	_ = st.EnableSecrets("00000000000000000000000000000000000000000000000000000000000000dd")
	if _, err := st.AddSubscriptionURLs([]string{upA.URL, upB.URL}); err != nil {
		t.Fatal(err)
	}

	in := NewInternal(config.Config{}, st)
	rr := httptest.NewRecorder()
	in.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/sub", nil))
	if rr.Code != 200 {
		t.Fatalf("code %d", rr.Code)
	}
	dec, err := base64.StdEncoding.DecodeString(rr.Body.String())
	if err != nil {
		t.Fatalf("not base64: %v", err)
	}
	var lines []string
	for _, l := range strings.Split(string(dec), "\n") {
		if strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
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
}
