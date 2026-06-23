package webctl

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// серт переиспользуется между вызовами (не пересоздаётся на каждый старт).
func TestLoadOrCreateSelfSignedReuses(t *testing.T) {
	dir := t.TempDir()
	c1, err := loadOrCreateSelfSigned(dir, "status.example.com")
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "self-signed.pem")); err != nil {
		t.Fatalf("серт не сохранён на диск: %v", err)
	}
	c2, err := loadOrCreateSelfSigned(dir, "status.example.com")
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if !bytes.Equal(c1.Certificate[0], c2.Certificate[0]) {
		t.Fatal("серт пересоздан, хотя должен переиспользоваться из кэша")
	}
}

// при смене домена кэш отбрасывается и серт перевыписывается.
func TestLoadOrCreateSelfSignedRegeneratesOnDomainChange(t *testing.T) {
	dir := t.TempDir()
	c1, err := loadOrCreateSelfSigned(dir, "old.example.com")
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	c2, err := loadOrCreateSelfSigned(dir, "new.example.com")
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if bytes.Equal(c1.Certificate[0], c2.Certificate[0]) {
		t.Fatal("серт не перевыписан при смене домена")
	}
	if c2.Leaf == nil || c2.Leaf.VerifyHostname("new.example.com") != nil {
		t.Fatal("новый серт не выписан на новый домен")
	}
}

// битый/просроченный кэш не валит загрузку — генерируется новый.
func TestLoadOrCreateSelfSignedCorruptCache(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "self-signed.pem"), []byte("garbage"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadOrCreateSelfSigned(dir, "status.example.com"); err != nil {
		t.Fatalf("битый кэш должен игнорироваться, got: %v", err)
	}
}
