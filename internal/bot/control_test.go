package bot

import (
	"strings"
	"testing"

	"xray-status/internal/config"
	"xray-status/internal/store"
	"xray-status/internal/storetest"
)

func TestCanControl(t *testing.T) {
	// Пусто => всё включено.
	tb := &Bot{cfg: config.Config{}}
	if !tb.canControl("subscription") {
		t.Fatal("пустой CONTROL_CAPS => управление включено")
	}
	// Задан без нужного capability => выключено; перечисленный => включён.
	tb2 := &Bot{cfg: config.Config{ControlCaps: []string{"maintenance"}}}
	if tb2.canControl("subscription") {
		t.Fatal("caps без subscription => выключено")
	}
	if !tb2.canControl("maintenance") {
		t.Fatal("перечисленный cap => включён")
	}
}

func TestCmdAudit(t *testing.T) {
	st, err := store.Open(storetest.DSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	tb := &Bot{st: st, cfg: config.Config{TZ: "UTC"}}
	if out := cmdAudit(tb); !strings.Contains(out, "пуст") {
		t.Fatalf("пустой журнал: %s", out)
	}
	_ = st.AddAudit(1, "server_hide", "DE", "", "ok")
	if out := cmdAudit(tb); !strings.Contains(out, "server_hide") {
		t.Fatalf("журнал должен фиксировать действие: %s", out)
	}
}
