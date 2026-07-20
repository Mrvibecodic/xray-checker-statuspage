package bot

import (
	"path/filepath"
	"strings"
	"testing"

	"xray-status/internal/config"
	"xray-status/internal/store"
)

func TestCmdSet(t *testing.T) {
	st, err := store.Open("sqlite", filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	if out := cmdSet(st, []string{"ping", "250"}); !strings.Contains(out, "250") {
		t.Fatalf("ping set: %s", out)
	}
	if st.PingThreshold() != 250 {
		t.Fatalf("ping threshold not persisted: %d", st.PingThreshold())
	}
	if out := cmdSet(st, []string{"summary", "9:30"}); !strings.Contains(out, "HH:MM") {
		t.Fatalf("bad time should be rejected: %s", out)
	}
	if out := cmdSet(st, []string{"summary", "09:30"}); !strings.Contains(out, "09:30") {
		t.Fatalf("good time: %s", out)
	}
	if st.DailySummaryTime() != "09:30" {
		t.Fatalf("summary time not persisted: %q", st.DailySummaryTime())
	}
	if out := cmdSet(st, []string{"alert_down", "off"}); !strings.Contains(out, "выкл") {
		t.Fatalf("alert off: %s", out)
	}
	if st.AlertOnDown() {
		t.Fatal("alert_on_down should be false")
	}
	cmdSet(st, []string{"domain", "https://Status.Example.com/"})
	if st.PublicDomain() != "status.example.com" {
		t.Fatalf("domain normalize failed: %q", st.PublicDomain())
	}
}

func TestCmdNginx(t *testing.T) {
	st, _ := store.Open("sqlite", filepath.Join(t.TempDir(), "t.db"))
	defer st.Close()
	_ = st.SetPublicDomain("status.example.com")
	out := cmdNginx(st, config.Config{Port: "8080"})
	if !strings.Contains(out, "server_name status.example.com") || !strings.Contains(out, "127.0.0.1:8080") {
		t.Fatalf("nginx config wrong:\n%s", out)
	}
}
