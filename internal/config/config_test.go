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

func TestTelegramProxy(t *testing.T) {
	t.Setenv("TELEGRAM_PROXY", "socks5://user:pass@127.0.0.1:1080")
	if got := Load().TelegramProxy; got != "socks5://user:pass@127.0.0.1:1080" {
		t.Fatalf("unexpected TelegramProxy: %q", got)
	}
}

func TestLocaleAndWebAuthEnv(t *testing.T) {
	for _, k := range []string{"LOCALE", "WEB_PASSWORD", "WEB_USER", "TITLE", "SUBTITLE"} {
		t.Setenv(k, "")
	}
	if c := Load(); c.Locale != "ru" || c.WebPassword != "" || c.WebUser != "" {
		t.Fatalf("defaults: got locale=%q user=%q pwd-set=%v", c.Locale, c.WebUser, c.WebPassword != "")
	} else if c.Title != "Статус серверов" || c.Subtitle != "Доступность серверов в реальном времени" {
		t.Fatalf("RU default text: title=%q subtitle=%q", c.Title, c.Subtitle)
	}
	t.Setenv("LOCALE", "en")
	t.Setenv("WEB_PASSWORD", "s3cret")
	t.Setenv("WEB_USER", "admin")
	c := Load()
	if c.Locale != "en" || c.WebPassword != "s3cret" || c.WebUser != "admin" {
		t.Fatalf("env not loaded: %+v", c)
	}
	if c.Title != "Server status" || c.Subtitle != "Real-time server availability" {
		t.Fatalf("EN default text: title=%q subtitle=%q", c.Title, c.Subtitle)
	}
	if !IsEnglish(c.Locale) || IsEnglish("ru") {
		t.Fatal("IsEnglish mismatch")
	}
}

func TestExplicitTitleAndSubtitleOverrideLocale(t *testing.T) {
	t.Setenv("LOCALE", "en")
	t.Setenv("TITLE", "My servers")
	t.Setenv("SUBTITLE", "Production checks")
	c := Load()
	if c.Title != "My servers" || c.Subtitle != "Production checks" {
		t.Fatalf("explicit text overwritten: title=%q subtitle=%q", c.Title, c.Subtitle)
	}
}
