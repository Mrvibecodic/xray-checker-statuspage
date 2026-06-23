package store

import (
	"path/filepath"
	"testing"
	"time"
)

func openTmp(t *testing.T) *Store {
	t.Helper()
	st, err := Open("sqlite", filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestIncidentLifecycle(t *testing.T) {
	st := openTmp(t)
	id, err := st.CreateIncident("API down", "major", []string{"NL", "DE"}, "Investigating", 1, false)
	if err != nil || id == 0 {
		t.Fatalf("create: id=%d err=%v", id, err)
	}
	act, err := st.ActiveIncidents()
	if err != nil || len(act) != 1 {
		t.Fatalf("active: %d %v", len(act), err)
	}
	if act[0].Title != "API down" || act[0].Status != "investigating" || len(act[0].Affected) != 2 {
		t.Fatalf("incident fields wrong: %+v", act[0])
	}
	if err := st.AddIncidentUpdate(id, "monitoring", "Fix deployed", 1); err != nil {
		t.Fatal(err)
	}
	if err := st.AddIncidentUpdate(id, "resolved", "All good", 1); err != nil {
		t.Fatal(err)
	}
	act, _ = st.ActiveIncidents()
	if len(act) != 0 {
		t.Fatalf("resolved incident still active: %d", len(act))
	}
	in, _ := st.GetIncident(id)
	if in.Status != "resolved" || in.ResolvedTS == 0 {
		t.Fatalf("resolved_ts not set: %+v", in)
	}
	ups, _ := st.IncidentUpdates(id)
	if len(ups) != 3 { // investigating + monitoring + resolved
		t.Fatalf("want 3 updates, got %d", len(ups))
	}
}

func TestMaintenanceWindow(t *testing.T) {
	st := openTmp(t)
	now := time.Now().Unix()
	id, err := st.AddMaintenance("NL", now-10, now+3600, "upgrade", 1)
	if err != nil || id == 0 {
		t.Fatalf("add: %v", err)
	}
	names, _ := st.MaintenanceNames(now)
	if !names["NL"] {
		t.Fatalf("NL should be in maintenance: %v", names)
	}
	// вне окна
	future, _ := st.MaintenanceNames(now + 7200)
	if future["NL"] {
		t.Fatalf("NL should be out of maintenance after window")
	}
	// закрыли вручную
	if err := st.EndMaintenance(id); err != nil {
		t.Fatal(err)
	}
	names2, _ := st.MaintenanceNames(now)
	if names2["NL"] {
		t.Fatalf("NL should be cleared after EndMaintenance")
	}
}
