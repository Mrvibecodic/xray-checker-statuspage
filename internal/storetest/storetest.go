// Package storetest — общий помощник для тестов, открывающих БД.
//
// По умолчанию (как раньше) тесты идут на SQLite во временном файле. Если задана
// переменная окружения TEST_PG_DSN, те же тесты идут на Postgres: каждый вызов
// DSN получает собственную схему, которая удаляется после теста, поэтому тесты
// не мешают друг другу и не требуют чистой базы.
//
// Пакет намеренно не импортирует internal/store — иначе внутренние тесты самого
// store не смогли бы им пользоваться (цикл импортов).
package storetest

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	_ "github.com/lib/pq"
)

var schemaSeq atomic.Int64

// DSN возвращает аргументы для store.Open: драйвер и строку подключения.
// Использование: st, err := store.Open(storetest.DSN(t)).
func DSN(t testing.TB) (string, string) {
	t.Helper()
	base := os.Getenv("TEST_PG_DSN")
	if base == "" {
		return "sqlite", t.TempDir() + "/t.db"
	}

	schema := fmt.Sprintf("t%d_%d", os.Getpid(), schemaSeq.Add(1))
	admin, err := sql.Open("postgres", base)
	if err != nil {
		t.Fatalf("открыть админ-соединение к Postgres: %v", err)
	}
	defer admin.Close()
	if _, err := admin.Exec(`CREATE SCHEMA "` + schema + `"`); err != nil {
		t.Fatalf("создать схему %s: %v", schema, err)
	}
	t.Cleanup(func() {
		db, err := sql.Open("postgres", base)
		if err != nil {
			return
		}
		defer db.Close()
		_, _ = db.Exec(`DROP SCHEMA IF EXISTS "` + schema + `" CASCADE`)
	})
	return "postgres", withSearchPath(base, schema)
}

// withSearchPath добавляет в DSN параметр search_path, чтобы весь DDL и все
// запросы теста работали внутри своей схемы. Поддерживаются обе формы DSN,
// понятные lib/pq: URL и «ключ=значение».
func withSearchPath(dsn, schema string) string {
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		sep := "?"
		if strings.Contains(dsn, "?") {
			sep = "&"
		}
		return dsn + sep + "search_path=" + schema
	}
	return dsn + " search_path=" + schema
}
