// Package store — слой доступа к БД. Поддерживает SQLite (modernc, без CGO,
// дефолт) и Postgres (lib/pq) для производительности; выбор через DB_DRIVER.
//
// Таблицы current/daily/samples/hidden/settings — формат совместим со старой
// Python-БД (SQLite), миграция бесшовная. Запросы пишутся в стиле SQLite (`?`),
// dialect.rebind переводит под Postgres.
package store

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"  // postgres driver ("postgres")
	_ "modernc.org/sqlite" // sqlite driver ("sqlite"), без CGO

	"xray-status/internal/secret"
)

type Store struct {
	db  *sql.DB
	d   dialect
	box *secret.Box
}

// Open открывает БД нужного драйвера.
//   - driver="sqlite": dsn = путь к файлу (включается WAL, одно соединение);
//   - driver="postgres": dsn = строка подключения lib/pq.
func Open(driver, dsn string) (*Store, error) {
	var (
		db  *sql.DB
		err error
		d   dialect
	)
	switch driver {
	case "", "sqlite", "sqlite3":
		d = dialect{name: "sqlite"}
		db, err = sql.Open("sqlite", dsn+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
		if err == nil {
			db.SetMaxOpenConns(1) // сериализуем доступ — для нашей нагрузки достаточно
		}
	case "postgres", "postgresql", "pg":
		d = dialect{name: "postgres"}
		db, err = sql.Open("postgres", dsn)
		if err == nil {
			db.SetMaxOpenConns(10)
		}
	default:
		return nil, fmt.Errorf("неизвестный DB_DRIVER %q (ожидается sqlite или postgres)", driver)
	}
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping %s: %w", d.name, err)
	}
	s := &Store{db: db, d: d}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

// Driver возвращает имя активного драйвера ("sqlite"|"postgres").
func (s *Store) Driver() string { return s.d.name }

// EnableSecrets включает шифрование секретов ключом SECRET_KEY (hex, 32 байта).
func (s *Store) EnableSecrets(hexKey string) error {
	b, err := secret.New(hexKey)
	if err != nil {
		return err
	}
	s.box = b
	return nil
}

// SecretsEnabled — задан ли ключ шифрования.
func (s *Store) SecretsEnabled() bool { return s.box != nil }

// --- обёртки доступа: прогоняют запрос через rebind под нужный диалект ---

func (s *Store) exec(q string, a ...any) (sql.Result, error) { return s.db.Exec(s.d.rebind(q), a...) }
func (s *Store) query(q string, a ...any) (*sql.Rows, error) { return s.db.Query(s.d.rebind(q), a...) }
func (s *Store) queryRow(q string, a ...any) *sql.Row        { return s.db.QueryRow(s.d.rebind(q), a...) }

func (s *Store) begin() (*txw, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	return &txw{Tx: tx, d: s.d}, nil
}

// insertID вставляет строку и возвращает её id (Postgres — через RETURNING id).
func (s *Store) insertID(q string, a ...any) (int64, error) {
	if s.d.isPG() {
		var id int64
		err := s.queryRow(q+" RETURNING id", a...).Scan(&id)
		return id, err
	}
	res, err := s.exec(q, a...)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	return id, nil
}

func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS current(
			sid TEXT PRIMARY KEY, name TEXT, online BIGINT,
			latency BIGINT, ts BIGINT, seq BIGINT, grp TEXT)`,
		`CREATE TABLE IF NOT EXISTS daily(
			day TEXT, sid TEXT, up BIGINT DEFAULT 0, down BIGINT DEFAULT 0,
			lat_sum BIGINT DEFAULT 0, lat_cnt BIGINT DEFAULT 0,
			down_conf BIGINT DEFAULT 0, PRIMARY KEY(day, sid))`,
		`CREATE TABLE IF NOT EXISTS samples(
			ts BIGINT, sid TEXT, online BIGINT, latency BIGINT)`,
		`CREATE INDEX IF NOT EXISTS idx_samples ON samples(sid, ts)`,
		`CREATE TABLE IF NOT EXISTS hidden(name TEXT PRIMARY KEY)`,
		`CREATE TABLE IF NOT EXISTS muted(name TEXT PRIMARY KEY)`,
		`CREATE TABLE IF NOT EXISTS settings(k TEXT PRIMARY KEY, v TEXT)`,
		`CREATE TABLE IF NOT EXISTS maintenance(
			id ` + s.d.pk() + `,
			name TEXT, from_ts BIGINT, to_ts BIGINT,
			reason TEXT, created_by BIGINT, created_ts BIGINT)`,
		`CREATE TABLE IF NOT EXISTS incidents(
			id ` + s.d.pk() + `,
			title TEXT, severity TEXT, status TEXT, affected TEXT,
			started_ts BIGINT, resolved_ts BIGINT,
			auto BIGINT DEFAULT 0, created_by BIGINT)`,
		`CREATE TABLE IF NOT EXISTS incident_updates(
			id ` + s.d.pk() + `,
			incident_id BIGINT, ts BIGINT, status TEXT, body TEXT, author BIGINT)`,
		`CREATE TABLE IF NOT EXISTS secrets(
			k TEXT PRIMARY KEY, nonce ` + s.d.blob() + `, ciphertext ` + s.d.blob() + `)`,
		`CREATE TABLE IF NOT EXISTS audit(
			id ` + s.d.pk() + `, ts BIGINT,
			actor BIGINT, action TEXT, target TEXT, params TEXT, result TEXT)`,
		`CREATE TABLE IF NOT EXISTS bot_state(
			chat_id BIGINT, key TEXT, value TEXT, PRIMARY KEY(chat_id, key))`,
		`CREATE TABLE IF NOT EXISTS assets(
			k TEXT PRIMARY KEY, mime TEXT, data ` + s.d.blob() + `)`,
	}
	for _, q := range stmts {
		if _, err := s.exec(q); err != nil {
			return fmt.Errorf("migrate: %w (%s)", err, q)
		}
	}
	// Совместимость со старой SQLite-БД без down_conf.
	if s.d.isPG() {
		_, _ = s.exec(`ALTER TABLE daily ADD COLUMN IF NOT EXISTS down_conf BIGINT DEFAULT 0`)
	} else {
		_, _ = s.exec(`ALTER TABLE daily ADD COLUMN down_conf BIGINT DEFAULT 0`)
	}
	// Совместимость со старой БД без grp (имя группы балансира, чекер >=1.3.0).
	if s.d.isPG() {
		_, _ = s.exec(`ALTER TABLE current ADD COLUMN IF NOT EXISTS grp TEXT`)
	} else {
		_, _ = s.exec(`ALTER TABLE current ADD COLUMN grp TEXT`)
	}
	return nil
}

// --- settings ---

func (s *Store) GetSetting(k, def string) string {
	var v string
	if err := s.queryRow(`SELECT v FROM settings WHERE k=?`, k).Scan(&v); err != nil {
		return def
	}
	return v
}

func (s *Store) SetSetting(k, v string) error {
	_, err := s.exec(
		`INSERT INTO settings(k,v) VALUES(?,?) ON CONFLICT(k) DO UPDATE SET v=excluded.v`, k, v)
	return err
}

func (s *Store) LastPollTS() int64 {
	var v sql.NullString
	if err := s.queryRow(`SELECT v FROM settings WHERE k='last_poll_ts'`).Scan(&v); err != nil {
		return 0
	}
	var n int64
	_, _ = fmt.Sscan(v.String, &n)
	return n
}

// --- типы строк для summary ---

type CurrentRow struct {
	SID, Name string
	Grp       string // имя балансировочной группы; пусто => самостоятельный сервер
	Online    int
	Latency   int
	TS        int64
	Seq       int
}

type DailyRow struct {
	SID, Day                           string
	Up, Down, LatSum, LatCnt, DownConf int
}

type SampleRow struct {
	TS      int64
	Online  int
	Latency int
}

func (s *Store) CurrentRows() ([]CurrentRow, error) {
	rows, err := s.query(`SELECT sid,name,COALESCE(grp,''),online,latency,ts,seq FROM current ORDER BY seq`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CurrentRow
	for rows.Next() {
		var r CurrentRow
		if err := rows.Scan(&r.SID, &r.Name, &r.Grp, &r.Online, &r.Latency, &r.TS, &r.Seq); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) DailyRows() ([]DailyRow, error) {
	rows, err := s.query(`SELECT sid,day,up,down,lat_sum,lat_cnt,down_conf FROM daily`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DailyRow
	for rows.Next() {
		var r DailyRow
		if err := rows.Scan(&r.SID, &r.Day, &r.Up, &r.Down, &r.LatSum, &r.LatCnt, &r.DownConf); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) HiddenSet() (map[string]bool, error) {
	rows, err := s.query(`SELECT name FROM hidden`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out[n] = true
	}
	return out, rows.Err()
}

// GroupMembers возвращает все sid'ы той же группы (по name), что и переданный sid.
func (s *Store) GroupMembers(sid string) ([]string, error) {
	var name string
	if err := s.queryRow(`SELECT name FROM current WHERE sid=?`, sid).Scan(&name); err != nil {
		return []string{sid}, nil
	}
	rows, err := s.query(`SELECT sid FROM current WHERE name=?`, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var x string
		if err := rows.Scan(&x); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	if len(out) == 0 {
		out = []string{sid}
	}
	return out, rows.Err()
}

// SamplesForGroup тянет samples всех членов группы за [ds, end).
func (s *Store) SamplesForGroup(members []string, ds, end int64) ([]SampleRow, error) {
	if len(members) == 0 {
		return nil, nil
	}
	q := `SELECT ts,online,latency FROM samples WHERE sid IN (`
	args := make([]any, 0, len(members)+2)
	for i, m := range members {
		if i > 0 {
			q += ","
		}
		q += "?"
		args = append(args, m)
	}
	q += `) AND ts>=? AND ts<? ORDER BY ts`
	args = append(args, ds, end)
	rows, err := s.query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SampleRow
	for rows.Next() {
		var r SampleRow
		if err := rows.Scan(&r.TS, &r.Online, &r.Latency); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
