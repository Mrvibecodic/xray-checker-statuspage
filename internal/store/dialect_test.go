package store

import "testing"

func TestRebind(t *testing.T) {
	sq := dialect{name: "sqlite"}
	pg := dialect{name: "postgres"}
	q := `INSERT INTO t(a,b,c) VALUES(?,?,?) ON CONFLICT(a) DO UPDATE SET b=?`
	if got := sq.rebind(q); got != q {
		t.Errorf("sqlite must not change query:\n%s", got)
	}
	want := `INSERT INTO t(a,b,c) VALUES($1,$2,$3) ON CONFLICT(a) DO UPDATE SET b=$4`
	if got := pg.rebind(q); got != want {
		t.Errorf("pg rebind:\n got=%s\nwant=%s", got, want)
	}
}

func TestDialectTypes(t *testing.T) {
	if (dialect{name: "sqlite"}).pk() != "INTEGER PRIMARY KEY AUTOINCREMENT" {
		t.Error("sqlite pk")
	}
	if (dialect{name: "postgres"}).pk() != "BIGSERIAL PRIMARY KEY" {
		t.Error("pg pk")
	}
	if (dialect{name: "postgres"}).blob() != "BYTEA" || (dialect{name: "sqlite"}).blob() != "BLOB" {
		t.Error("blob types")
	}
}
