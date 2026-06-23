package store

import (
	"path/filepath"
	"testing"
)

func TestSubscriptionAndServerMeta(t *testing.T) {
	st, err := Open("sqlite", filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	_ = st.EnableSecrets("00000000000000000000000000000000000000000000000000000000000000bb")

	if st.HasSubscriptionURL() {
		t.Fatal("no sub url yet")
	}
	if err := st.SetSubscriptionURL("https://provider/sub?token=abc"); err != nil {
		t.Fatal(err)
	}
	if !st.HasSubscriptionURL() {
		t.Fatal("sub url should be set")
	}
	got, _ := st.SubscriptionURL()
	if got != "https://provider/sub?token=abc" {
		t.Fatalf("sub url: %q", got)
	}

	if err := st.SetServerEnabled("DE Frankfurt", false); err != nil {
		t.Fatal(err)
	}
	dis, _ := st.DisabledServers()
	if !dis["DE Frankfurt"] {
		t.Fatalf("DE should be disabled: %v", dis)
	}
	_ = st.SetServerEnabled("DE Frankfurt", true)
	dis2, _ := st.DisabledServers()
	if dis2["DE Frankfurt"] {
		t.Fatal("DE should be re-enabled")
	}
}
