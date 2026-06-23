package store

import (
	"path/filepath"
	"testing"
)

func TestBotState(t *testing.T) {
	st, err := Open("sqlite", filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if st.GetBotState(1, "await") != "" {
		t.Fatal("empty expected")
	}
	_ = st.SetBotState(1, "await", "domain")
	if st.GetBotState(1, "await") != "domain" {
		t.Fatal("set/get failed")
	}
	_ = st.DelBotState(1, "await")
	if st.GetBotState(1, "await") != "" {
		t.Fatal("del failed")
	}
}
