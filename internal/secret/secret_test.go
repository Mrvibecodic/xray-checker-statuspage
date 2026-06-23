package secret

import (
	"strings"
	"testing"
)

const k1 = "0000000000000000000000000000000000000000000000000000000000000000"
const k2 = "1111111111111111111111111111111111111111111111111111111111111111"

func TestRoundTrip(t *testing.T) {
	b, err := New(k1)
	if err != nil {
		t.Fatal(err)
	}
	nonce, ct, err := b.Encrypt([]byte("vless://secret@host:443"))
	if err != nil {
		t.Fatal(err)
	}
	pt, err := b.Decrypt(nonce, ct)
	if err != nil || string(pt) != "vless://secret@host:443" {
		t.Fatalf("round-trip failed: %q %v", pt, err)
	}
}

func TestWrongKeyFails(t *testing.T) {
	b1, _ := New(k1)
	b2, _ := New(k2)
	nonce, ct, _ := b1.Encrypt([]byte("x"))
	if _, err := b2.Decrypt(nonce, ct); err == nil {
		t.Fatal("decrypt with wrong key must fail")
	}
}

func TestBadKey(t *testing.T) {
	if _, err := New("zzzz"); err == nil {
		t.Error("non-hex key accepted")
	}
	if _, err := New("00"); err == nil || !strings.Contains(err.Error(), "32") {
		t.Error("short key should be rejected with length hint")
	}
}

func TestEnsureKeyFile(t *testing.T) {
	dir := t.TempDir()
	p := dir + "/secret.key"
	k1, err := EnsureKeyFile(p)
	if err != nil || len(k1) != 64 {
		t.Fatalf("gen key: %q %v", k1, err)
	}
	if _, e := New(k1); e != nil {
		t.Fatalf("generated key invalid: %v", e)
	}
	k2, _ := EnsureKeyFile(p) // повторный вызов — тот же ключ
	if k1 != k2 {
		t.Fatalf("key not stable: %q vs %q", k1, k2)
	}
}
