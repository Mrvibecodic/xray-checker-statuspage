package store

import (
	"testing"

	"xray-status/internal/checker"
	"xray-status/internal/storetest"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(storetest.DSN(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestDeleteServer_balancerGroupWholeGroup(t *testing.T) {
	st := openTestStore(t)
	if _, err := st.PollWrite([]checker.Proxy{
		{StableID: "g1", Name: "Bal | proxy", GroupName: "Bal", Online: true},
		{StableID: "g2", Name: "Bal | proxy-2", GroupName: "Bal", Online: true},
		{StableID: "g3", Name: "Bal | proxy-3", GroupName: "Bal", Online: false},
		{StableID: "solo", Name: "Solo", Online: true},
	}, PollWriteParams{Now: 1_700_000_000, Today: "2026-07-24", PollInterval: 60}); err != nil {
		t.Fatal(err)
	}

	n, err := st.DeleteServer("g2")
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("ожидалось удаление всех 3 узлов группы, удалено %d", n)
	}

	rows, err := st.CurrentRows()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].SID != "solo" {
		t.Fatalf("должен остаться только solo, got %+v", rows)
	}
}

func TestDeleteServer_sameNameStillGrouped(t *testing.T) {
	st := openTestStore(t)
	if _, err := st.PollWrite([]checker.Proxy{
		{StableID: "a1", Name: "Same", Online: true},
		{StableID: "a2", Name: "Same", Online: true},
		{StableID: "b", Name: "Other", Online: true},
	}, PollWriteParams{Now: 1_700_000_000, Today: "2026-07-24", PollInterval: 60}); err != nil {
		t.Fatal(err)
	}
	n, err := st.DeleteServer("a1")
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("одноимённые sid должны удаляться вместе, удалено %d", n)
	}
	rows, _ := st.CurrentRows()
	if len(rows) != 1 || rows[0].SID != "b" {
		t.Fatalf("должен остаться только Other, got %+v", rows)
	}
}
