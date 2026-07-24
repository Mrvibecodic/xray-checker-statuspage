package bot

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-telegram/bot/models"

	"xray-status/internal/checker"
	"xray-status/internal/config"
	"xray-status/internal/store"
)

// Меню видимости/мьюта/обслуживания/инцидентов должны показывать балансир-группу
// ОДНОЙ кнопкой с именем группы (как на странице и в панели статуса), а не
// узлами «… | proxy», «… | proxy-2»; действие применяется к группе целиком.
func newGroupBot(t *testing.T) *Bot {
	t.Helper()
	st, err := store.Open("sqlite", filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if _, err := st.PollWrite([]checker.Proxy{
		{StableID: "g1", Name: "Bal | proxy", GroupName: "Bal", Online: true},
		{StableID: "g2", Name: "Bal | proxy-2", GroupName: "Bal", Online: true},
		{StableID: "solo", Name: "Solo", Online: true},
	}, store.PollWriteParams{Now: 1_700_000_000, Today: "2026-07-24", PollInterval: 60}); err != nil {
		t.Fatal(err)
	}
	return &Bot{
		st:     st,
		cfg:    config.Config{Title: "T", Days: 30, TZ: "Europe/Moscow", PollInterval: 60, InternalPort: "8081"},
		admins: map[int64]bool{1: true},
	}
}

// flatKB — все подписи кнопок клавиатуры одной строкой (для проверок).
func flatKB(kb *models.InlineKeyboardMarkup) string {
	if kb == nil {
		return ""
	}
	var parts []string
	for _, row := range kb.InlineKeyboard {
		for _, b := range row {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, " | ")
}

func TestGroupNames_collapsesBalancer(t *testing.T) {
	tb := newGroupBot(t)
	names := tb.groupNames()
	if len(names) != 2 || names[0] != "Bal" || names[1] != "Solo" {
		t.Fatalf("groupNames: %v", names)
	}
	if got := tb.resolveName(nameTok("Bal")); got != "Bal" {
		t.Fatalf("resolveName группы: %q", got)
	}
	if got := tb.resolveName(nameTok("Bal | proxy")); got != "" {
		t.Fatalf("per-node токен не должен резолвиться, got %q", got)
	}
}

func TestMenus_showGroupOnce(t *testing.T) {
	tb := newGroupBot(t)
	for _, tc := range []struct {
		menu string
		flat string
	}{
		{"vis", flatKB(tb.visKB(1))},
		{"mute", flatKB(tb.muteKB(1))},
		{"maint", flatKB(tb.maintServersKB(1))},
		{"incAff", flatKB(tb.incAffKB(1))},
	} {
		if strings.Contains(tc.flat, "proxy") {
			t.Errorf("%s: в меню просочились узлы балансира: %s", tc.menu, tc.flat)
		}
		if strings.Count(tc.flat, "Bal") != 1 {
			t.Errorf("%s: группа должна быть ровно одной кнопкой: %s", tc.menu, tc.flat)
		}
		if !strings.Contains(tc.flat, "Solo") {
			t.Errorf("%s: одиночный сервер пропал: %s", tc.menu, tc.flat)
		}
	}
}

func TestVisMuteToggle_applyToGroup(t *testing.T) {
	tb := newGroupBot(t)
	tb.handleVisCallback(1, "vis:"+nameTok("Bal"))
	hidden, _ := tb.st.HiddenSet()
	if !hidden["Bal"] {
		t.Fatal("скрытие по кнопке должно лечь на имя группы")
	}
	tb.handleMuteCallback(1, "mute:"+nameTok("Bal"))
	muted, _ := tb.st.MutedSet()
	if !muted["Bal"] {
		t.Fatal("мьют по кнопке должен лечь на имя группы")
	}
}
