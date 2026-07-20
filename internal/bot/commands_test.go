package bot

import (
	"path/filepath"
	"strings"
	"testing"

	"xray-status/internal/store"
)

func tmpStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open("sqlite", filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestCmdIncidentFlow(t *testing.T) {
	st := tmpStore(t)
	if out := cmdIncident(st, []string{"new", "major", "API", "недоступно"}, 1); !strings.Contains(out, "#1") {
		t.Fatalf("new: %s", out)
	}
	if out := cmdIncident(st, []string{"new", "wrong", "x"}, 1); !strings.Contains(out, "severity") {
		t.Fatalf("bad severity not rejected: %s", out)
	}
	if incs, _ := st.ActiveIncidents(); len(incs) != 1 || !strings.Contains(incs[0].Title, "API") {
		t.Fatalf("active incidents: %v", incs)
	}
	if out := cmdIncident(st, []string{"update", "1", "monitoring", "фиксим"}, 1); !strings.Contains(out, "monitoring") {
		t.Fatalf("update: %s", out)
	}
	if out := cmdIncident(st, []string{"resolve", "1", "ок"}, 1); !strings.Contains(out, "закрыт") {
		t.Fatalf("resolve: %s", out)
	}
	if incs, _ := st.ActiveIncidents(); len(incs) != 0 {
		t.Fatalf("should be empty after resolve: %v", incs)
	}
}
