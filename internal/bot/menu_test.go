package bot

import (
	"strings"
	"testing"

	"xray-status/internal/config"
	"xray-status/internal/store"
	"xray-status/internal/storetest"
)

func TestSectionText(t *testing.T) {
	st, err := store.Open(storetest.DSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	tb := &Bot{st: st, cfg: config.Config{Title: "T", Days: 30, TZ: "UTC", PollInterval: 60},
		admins: map[int64]bool{1: true}}

	if txt, kb := tb.sectionText(1, "m:home"); !strings.Contains(txt, "Панель") || kb == nil {
		t.Errorf("home: %q kb=%v", txt, kb)
	}
	if txt, _ := tb.sectionText(1, "m:settings"); !strings.Contains(txt, "Настройки") {
		t.Errorf("settings section: %s", txt)
	}
	if txt, _ := tb.sectionText(1, "m:more"); !strings.Contains(txt, "Ещё") {
		t.Errorf("more section: %s", txt)
	}
	if txt, _ := tb.sectionText(1, "m:nginx"); !strings.Contains(txt, "nginx") {
		t.Errorf("nginx section: %s", txt)
	}
	// у листового раздела должна быть кнопка «Назад»
	if _, kb := tb.sectionText(1, "m:status"); kb == nil || len(kb.InlineKeyboard) == 0 {
		t.Error("status section missing back keyboard")
	}
}
