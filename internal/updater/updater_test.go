package updater

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateELF(t *testing.T) {
	if err := ValidateELF([]byte{0x7f, 'E', 'L', 'F', 0, 1}); err != nil {
		t.Errorf("valid ELF rejected: %v", err)
	}
	if ValidateELF([]byte("not elf")) == nil {
		t.Error("non-ELF accepted")
	}
}

func TestFetchAndReplace(t *testing.T) {
	elf := append([]byte{0x7f, 'E', 'L', 'F'}, make([]byte, 200)...)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(elf)
	}))
	defer srv.Close()

	u := New(srv.URL, "")
	if !u.Available() {
		t.Fatal("should be available with url")
	}
	data, err := u.Fetch(context.Background())
	if err != nil || !bytes.Equal(data, elf) {
		t.Fatalf("fetch: err=%v len=%d", err, len(data))
	}
	if err := ValidateELF(data); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	target := filepath.Join(dir, "statuspage")
	if err := os.WriteFile(target, []byte("OLDBINARY"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := ReplaceExecutable(target, data); err != nil {
		t.Fatalf("replace: %v", err)
	}
	got, _ := os.ReadFile(target)
	if !bytes.Equal(got, data) {
		t.Error("target not replaced with new data")
	}
	if fi, _ := os.Stat(target); fi.Mode().Perm()&0o100 == 0 {
		t.Error("target not executable")
	}
	if _, err := os.Stat(target + ".old"); !os.IsNotExist(err) {
		t.Error(".old backup should be cleaned on success")
	}
}

func TestNotAvailable(t *testing.T) {
	if New("", "").Available() {
		t.Error("empty url must be unavailable")
	}
}

func TestCheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"sha":"abcdef1234567","commit":{"message":"fix: a thing\n\nbody here"}}`))
	}))
	defer srv.Close()
	u := New("http://x", "")
	u.commitsAPI = srv.URL

	res, err := u.Check(context.Background(), "go-build-0000000")
	if err != nil {
		t.Fatal(err)
	}
	if !res.HasUpdate || res.Latest != "abcdef1" || res.Message != "fix: a thing" {
		t.Fatalf("check: %+v", res)
	}
	// текущая версия уже содержит этот sha -> обновления нет
	res2, _ := u.Check(context.Background(), "go-build-abcdef1")
	if res2.HasUpdate {
		t.Fatal("should report up-to-date")
	}
}
