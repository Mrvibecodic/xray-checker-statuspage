package store

import (
	"database/sql"
	"strconv"
	"strings"
)

// dialect инкапсулирует различия SQLite и Postgres: плейсхолдеры, типы в DDL,
// получение id вставленной строки. Запросы во всём пакете пишутся в стиле SQLite
// (плейсхолдер `?`), а rebind переводит их под Postgres ($1,$2,...).
type dialect struct {
	name string // "sqlite" | "postgres"
}

func (d dialect) isPG() bool { return d.name == "postgres" }

// rebind переводит `?` в `$1,$2,...` для Postgres. В наших запросах `?` не
// встречается внутри строковых литералов, поэтому замена безопасна.
func (d dialect) rebind(q string) string {
	if !d.isPG() {
		return q
	}
	var b strings.Builder
	b.Grow(len(q) + 8)
	n := 0
	for i := 0; i < len(q); i++ {
		if q[i] == '?' {
			n++
			b.WriteByte('$')
			b.WriteString(strconv.Itoa(n))
		} else {
			b.WriteByte(q[i])
		}
	}
	return b.String()
}

// pk — объявление автоинкрементного первичного ключа.
func (d dialect) pk() string {
	if d.isPG() {
		return "BIGSERIAL PRIMARY KEY"
	}
	return "INTEGER PRIMARY KEY AUTOINCREMENT"
}

// blob — тип для бинарных данных (секреты).
func (d dialect) blob() string {
	if d.isPG() {
		return "BYTEA"
	}
	return "BLOB"
}

// txw — обёртка над *sql.Tx, прозрачно прогоняющая запросы через rebind.
// Методы Commit/Rollback промоутятся из встроенного *sql.Tx.
type txw struct {
	*sql.Tx
	d dialect
}

func (t *txw) Exec(q string, a ...any) (sql.Result, error) { return t.Tx.Exec(t.d.rebind(q), a...) }
func (t *txw) Query(q string, a ...any) (*sql.Rows, error) { return t.Tx.Query(t.d.rebind(q), a...) }
func (t *txw) QueryRow(q string, a ...any) *sql.Row        { return t.Tx.QueryRow(t.d.rebind(q), a...) }
