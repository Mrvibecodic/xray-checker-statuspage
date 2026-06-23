package config

import (
	"os"
	"testing"
)

// CONTROL_CAPS не задан => управление полностью включено (caps пуст), а не
// принудительно ["maintenance"]. Регрессия: из-за форсинга подписка/контроль
// были выключены из коробки.
func TestControlCapsEmptyByDefault(t *testing.T) {
	os.Unsetenv("CONTROL_CAPS")
	if c := Load(); len(c.ControlCaps) != 0 {
		t.Fatalf("ControlCaps must be empty by default, got %v", c.ControlCaps)
	}
}

func TestControlCapsExplicit(t *testing.T) {
	t.Setenv("CONTROL_CAPS", "maintenance, subscription")
	c := Load()
	if len(c.ControlCaps) != 2 || c.ControlCaps[0] != "maintenance" || c.ControlCaps[1] != "subscription" {
		t.Fatalf("unexpected caps: %v", c.ControlCaps)
	}
}

func TestTLSDefaults(t *testing.T) {
	for _, k := range []string{"TLS_MODE", "CERT_FILE", "KEY_FILE"} {
		os.Unsetenv(k)
	}
	c := Load()
	if c.TLSMode != "" || c.CertFile != "" || c.KeyFile != "" {
		t.Fatalf("tls defaults not empty: %q %q %q", c.TLSMode, c.CertFile, c.KeyFile)
	}
}
