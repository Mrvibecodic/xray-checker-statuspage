package store

import (
	"path/filepath"
	"testing"
)

func TestSecrets(t *testing.T) {
	st, err := Open("sqlite", filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	// без ключа — ошибка
	if _, err := st.GetSecret("sub"); err != ErrSecretsDisabled {
		t.Fatalf("expected ErrSecretsDisabled, got %v", err)
	}
	if err := st.EnableSecrets("00000000000000000000000000000000000000000000000000000000000000aa"); err != nil {
		t.Fatal(err)
	}
	if !st.SecretsEnabled() {
		t.Fatal("secrets should be enabled")
	}
	if err := st.SetSecret("sub", "vless://x@h:443#NL"); err != nil {
		t.Fatal(err)
	}
	if !st.HasSecret("sub") {
		t.Fatal("HasSecret should be true")
	}
	got, err := st.GetSecret("sub")
	if err != nil || got != "vless://x@h:443#NL" {
		t.Fatalf("get secret: %q %v", got, err)
	}
}
