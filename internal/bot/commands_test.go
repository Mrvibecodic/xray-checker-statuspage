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
	if out := cmdIncidents(st); !strings.Contains(out, "API") {
		t.Fatalf("list: %s", out)
	}
	if out := cmdIncident(st, []string{"update", "1", "monitoring", "фиксим"}, 1); !strings.Contains(out, "monitoring") {
		t.Fatalf("update: %s", out)
	}
	if out := cmdIncident(st, []string{"resolve", "1", "ок"}, 1); !strings.Contains(out, "закрыт") {
		t.Fatalf("resolve: %s", out)
	}
	if out := cmdIncidents(st); !strings.Contains(out, "нет") {
		t.Fatalf("should be empty after resolve: %s", out)
	}
}

func TestCmdMaintenance(t *testing.T) {
	st := tmpStore(t)
	out := cmdMaintenance(st, []string{"60", "DE", "Frankfurt", "|", "апгрейд"}, 1)
	if !strings.Contains(out, "DE Frankfurt") {
		t.Fatalf("schedule (name with spaces): %s", out)
	}
	if out := cmdMaintenance(st, nil, 1); !strings.Contains(out, "DE Frankfurt") {
		t.Fatalf("list active: %s", out)
	}
	if out := cmdMaintenance(st, []string{"off", "DE", "Frankfurt"}, 1); !strings.Contains(out, "завершены") {
		t.Fatalf("off: %s", out)
	}
	if out := cmdMaintenance(st, nil, 1); !strings.Contains(out, "нет") {
		t.Fatalf("should be empty: %s", out)
	}
}
