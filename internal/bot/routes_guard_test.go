package bot

import (
	"strings"
	"testing"

	"github.com/go-telegram/bot/models"

	"xray-status/internal/checker"
	"xray-status/internal/config"
	"xray-status/internal/store"
	"xray-status/internal/storetest"
)

// TestEveryButtonIsRouted — страховка от «мёртвых кнопок»: каждый callback,
// который конструирует ЛЮБАЯ клавиатура бота, обязан попадать в известный
// маршрут onCallback (точное действие, известный префикс обработчика или
// известная секция m:*). Появилась новая кнопка без маршрута — тест упадёт.
func TestEveryButtonIsRouted(t *testing.T) {
	st, err := store.Open(storetest.DSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := st.PollWrite([]checker.Proxy{
		{StableID: "a", Name: "Alpha", Online: true, LatencyMs: 10},
		{StableID: "b1", Name: "Beta | proxy", GroupName: "Beta", Online: true, LatencyMs: 20},
		{StableID: "b2", Name: "Beta | proxy-2", GroupName: "Beta", Online: false},
	}, store.PollWriteParams{Now: 1_700_000_000, Today: "2026-07-24", PollInterval: 60}); err != nil {
		t.Fatal(err)
	}
	incID, err := st.CreateIncident("Инцидент", "major", []string{"Alpha"}, "b", 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddMaintenance("Alpha", 1_700_000_000, 1_700_003_600, "r", 1); err != nil {
		t.Fatal(err)
	}
	_ = st.EnableSecrets("00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff")
	_ = st.SetSubscriptionURL("https://example.com/sub")

	tb := &Bot{
		st:     st,
		cfg:    config.Config{Title: "T", Days: 30, TZ: "UTC", PollInterval: 60, InternalPort: "8081", Port: "8080", HTTPSPort: "8443", ControlCaps: []string{"subscription"}},
		admins: map[int64]bool{1: true},
	}

	// Известные маршруты onCallback (см. bot.go). Новые добавлять сюда ЖЕ,
	// когда появляется обработчик.
	exact := map[string]bool{
		"noop": true, "m:doupdate": true, "m:restart": true, "m:upcheck": true,
		"m:webstart": true, "m:webstop": true, "m:refresh": true, "sub:diag": true,
	}
	prefixes := []string{"set:", "mnt:", "inc:", "sub:", "vis:", "mute:", "cl:"}
	sections := map[string]bool{
		"m:home": true, "m:more": true, "m:page": true, "m:clean": true, "m:nginx": true,
		"m:status": true, "m:servers": true, "m:stats": true, "m:incidents": true,
		"m:maint": true, "m:vis": true, "m:sub": true, "m:settings": true,
		"m:web": true, "m:audit": true, "m:update": true,
	}
	routed := func(cb string) bool {
		if exact[cb] || sections[cb] {
			return true
		}
		for _, p := range prefixes {
			if strings.HasPrefix(cb, p) {
				return true
			}
		}
		return false
	}

	kbs := map[string]*models.InlineKeyboardMarkup{
		"main":         mainMenuKB(),
		"more":         moreKB(),
		"update":       updateKB(),
		"moreBack":     moreBackKB(),
		"back":         backKB(),
		"vis":          tb.visKB(1),
		"sub":          tb.subKB(),
		"subDiag":      subDiagKB(),
		"settings":     tb.settingsKB(),
		"attl":         attlKB(),
		"theme":        themeKB(),
		"ping":         pingKB(),
		"summary":      summaryKB(),
		"mute":         tb.muteKB(1),
		"web":          tb.webKB(),
		"page":         tb.pageKB(),
		"pageCancel":   pageCancelKB(),
		"clean":        tb.cleanKB(),
		"cleanHours":   cleanHoursKB(),
		"absent":       tb.absentKB(1),
		"maint":        maintKB(tb.st, tb.cfg),
		"maintServers": tb.maintServersKB(1),
		"maintDur":     maintDurKB(),
		"incidents":    tb.incidentsKB(),
		"incSev":       incSevKB(),
		"incStatus":    incStatusKB(incID),
		"incAff":       tb.incAffKB(1),
	}
	for name, kb := range kbs {
		if kb == nil {
			t.Errorf("%s: nil-клавиатура", name)
			continue
		}
		for _, row := range kb.InlineKeyboard {
			for _, b := range row {
				if b.CallbackData == "" {
					t.Errorf("%s: кнопка %q без callback", name, b.Text)
					continue
				}
				if !routed(b.CallbackData) {
					t.Errorf("%s: кнопка %q → %q не имеет маршрута в onCallback", name, b.Text, b.CallbackData)
				}
			}
		}
	}
}
