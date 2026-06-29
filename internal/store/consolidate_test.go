package store

import "testing"

func insCur(t *testing.T, s *Store, sid, name string, ts int64) {
	t.Helper()
	if _, err := s.exec(`INSERT INTO current(sid,name,online,latency,ts,seq,grp) VALUES(?,?,?,?,?,?,NULL)`,
		sid, name, 1, 100, ts, 0); err != nil {
		t.Fatal(err)
	}
}
func insDay(t *testing.T, s *Store, day, sid string, up, down int) {
	t.Helper()
	if _, err := s.exec(`INSERT INTO daily(day,sid,up,down,lat_sum,lat_cnt,down_conf) VALUES(?,?,?,?,?,?,?)`,
		day, sid, up, down, up*100, up, down); err != nil {
		t.Fatal(err)
	}
}
func insSample(t *testing.T, s *Store, ts int64, sid string) {
	t.Helper()
	if _, err := s.exec(`INSERT INTO samples(ts,sid,online,latency) VALUES(?,?,?,?)`, ts, sid, 1, 100); err != nil {
		t.Fatal(err)
	}
}
func count(t *testing.T, s *Store, q string, a ...any) int {
	t.Helper()
	var n int
	if err := s.queryRow(q, a...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestConsolidateByName(t *testing.T) {
	s, err := Open("sqlite", t.TempDir()+"/c.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// 1) X: сменился stableId — old(ts5) отсутствует в опросе, new(ts10) актуален.
	insCur(t, s, "x_old", "X", 5)
	insCur(t, s, "x_new", "X", 10)
	insDay(t, s, "2026-06-06", "x_old", 5, 0)
	insDay(t, s, "2026-06-23", "x_old", 2, 3)
	insDay(t, s, "2026-06-23", "x_new", 1, 1)
	insDay(t, s, "2026-06-29", "x_new", 4, 0)
	insSample(t, s, 4, "x_old")
	insSample(t, s, 9, "x_new")

	// 2) Y: две РАЗНЫЕ подписки, одинаковое имя, ОБА активны (ts==lastPoll) — не трогать.
	insCur(t, s, "y_a", "Y", 10)
	insCur(t, s, "y_b", "Y", 10)
	insDay(t, s, "2026-06-29", "y_a", 3, 0)
	insDay(t, s, "2026-06-29", "y_b", 2, 1)

	// 3) Z: одиночный — no-op.
	insCur(t, s, "z1", "Z", 10)

	merged, err := s.ConsolidateByName(10)
	if err != nil {
		t.Fatal(err)
	}
	if merged != 1 {
		t.Fatalf("ожидалось 1 слияние (только X), got %d", merged)
	}

	// X: остался один current=x_new, x_old удалён
	if count(t, s, `SELECT count(*) FROM current WHERE name='X'`) != 1 {
		t.Fatal("X: должен остаться один current")
	}
	if count(t, s, `SELECT count(*) FROM current WHERE sid='x_old'`) != 0 {
		t.Fatal("X: x_old должен быть удалён")
	}
	// daily X переехал на x_new, день 23 просуммирован (2+1 up, 3+1 down)
	if count(t, s, `SELECT count(*) FROM daily WHERE sid='x_old'`) != 0 {
		t.Fatal("X: daily x_old должны переехать")
	}
	var up, down int
	if err := s.queryRow(`SELECT up,down FROM daily WHERE sid='x_new' AND day='2026-06-23'`).Scan(&up, &down); err != nil {
		t.Fatal(err)
	}
	if up != 3 || down != 4 {
		t.Fatalf("X день 23 не просуммирован: up=%d down=%d (ждали 3/4)", up, down)
	}
	if count(t, s, `SELECT count(distinct day) FROM daily WHERE sid='x_new'`) != 3 {
		t.Fatal("X: ожидалось 3 дня истории на x_new")
	}
	if count(t, s, `SELECT count(*) FROM samples WHERE sid='x_old'`) != 0 ||
		count(t, s, `SELECT count(*) FROM samples WHERE sid='x_new'`) != 2 {
		t.Fatal("X: samples должны перепривязаться на x_new")
	}

	// Y: оба активны — НЕ объединены, две строки остались
	if count(t, s, `SELECT count(*) FROM current WHERE name='Y'`) != 2 {
		t.Fatal("Y: одновременно активные одноимённые НЕ должны схлопываться")
	}

	// идемпотентность: второй прогон ничего не меняет
	m2, err := s.ConsolidateByName(10)
	if err != nil {
		t.Fatal(err)
	}
	if m2 != 0 {
		t.Fatalf("повторный прогон должен быть no-op, got %d", m2)
	}
}
