package store

import (
	"testing"
	"xray-status/internal/storetest"
)

func TestMutedServers(t *testing.T) {
	st, err := Open(storetest.DSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if m, _ := st.MutedSet(); len(m) != 0 {
		t.Fatalf("muted must be empty at start: %v", m)
	}
	if err := st.SetMutedName("DE Frankfurt", true); err != nil {
		t.Fatal(err)
	}
	m, _ := st.MutedSet()
	if !m["DE Frankfurt"] {
		t.Fatalf("DE must be muted: %v", m)
	}
	_ = st.SetMutedName("DE Frankfurt", false)
	if m2, _ := st.MutedSet(); m2["DE Frankfurt"] {
		t.Fatal("DE must be unmuted")
	}
}
