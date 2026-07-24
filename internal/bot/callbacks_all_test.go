package bot

import (
	"path/filepath"
	"testing"

	"xray-status/internal/checker"
	"xray-status/internal/config"
	"xray-status/internal/store"
)

// TestAllCallbacks прогоняет КАЖДЫЙ callback-обработчик бота с засеянными
// данными и проверяет, что он не паникует и возвращает непустой текст +
// клавиатуру. Страховка против «мёртвых кнопок».
func TestAllCallbacks(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open("sqlite", filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	if _, err := st.PollWrite([]checker.Proxy{
		{StableID: "de1", Name: "DE Frankfurt", Online: true, LatencyMs: 42},
		{StableID: "nl1", Name: "NL Amsterdam", Online: false, LatencyMs: 0},
	}, store.PollWriteParams{Now: 1_700_000_000, Today: "2026-06-23", PollInterval: 60}); err != nil {
		t.Fatal(err)
	}
	incID, err := st.CreateIncident("Тестовый инцидент", "major", []string{"DE Frankfurt"}, "body", 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddMaintenance("DE Frankfurt", 1_700_000_000, 1_700_003_600, "плановые", 1); err != nil {
		t.Fatal(err)
	}
	_ = st.EnableSecrets("00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff")
	_ = st.SetSubscriptionURL("https://example.com/sub")

	tb := &Bot{
		st:     st,
		cfg:    config.Config{Title: "T", Days: 30, TZ: "Europe/Moscow", PollInterval: 60, InternalPort: "8081", Port: "8080", HTTPSPort: "8443", ControlCaps: []string{"subscription"}},
		admins: map[int64]bool{1: true},
	}

	check := func(name, txt string, kb interface{}) {
		if txt == "" {
			t.Errorf("%s: пустой текст", name)
		}
		if kb == nil {
			t.Errorf("%s: nil-клавиатура", name)
		}
	}

	for _, d := range []string{"m:home", "m:more", "m:status", "m:servers", "m:stats", "m:incidents",
		"m:maint", "m:settings", "m:update", "m:web", "m:vis", "m:sub", "m:audit", "m:clean", "m:nginx", "m:page"} {
		txt, kb := tb.sectionText(1, d)
		check("section "+d, txt, kb)
	}

	for _, d := range []string{"set:home", "set:alert", "set:ping", "set:ping:300", "set:ping:0",
		"set:sum", "set:sum:09:00", "set:sum:off", "set:domain",
		"set:attl", "set:attl:24", "set:attl:0",
		"set:theme", "set:theme:dark",
		"set:title", "set:subtitle", "set:desc", "set:favicon", "set:favreset"} {
		txt, kb := tb.handleSettingCallback(1, 10, d)
		check("setting "+d, txt, kb)
	}

	for _, d := range []string{"mnt:new", "mnt:srvpg:0", "mnt:srv:DE Frankfurt", "mnt:dur:60", "mnt:end:DE Frankfurt"} {
		txt, kb := tb.handleMaintCallback(1, d)
		check("maint "+d, txt, kb)
	}

	idStr := itoa64(incID)
	for _, d := range []string{"inc:new", "inc:sev:major", "inc:open:" + idStr,
		"inc:setst:" + idStr + ":monitoring", "inc:resolve:" + idStr,
		"inc:aff:DE Frankfurt", "inc:affpg:0", "inc:affnone", "inc:affdone"} {
		txt, kb := tb.handleIncCallback(1, 10, d)
		check("inc "+d, txt, kb)
	}

	for _, d := range []string{"sub:add", "sub:del:0"} {
		txt, kb := tb.handleSubCallback(1, 10, d)
		check("sub "+d, txt, kb)
	}

	for _, d := range []string{"vis:DE Frankfurt", "vis:NL Amsterdam", "vis:pg:0", "vis:pg:1"} {
		txt, kb := tb.handleVisCallback(1, d)
		check("vis "+d, txt, kb)
	}

	for _, d := range []string{"mute:DE Frankfurt", "mute:pg:0"} {
		txt, kb := tb.handleMuteCallback(1, d)
		check("mute "+d, txt, kb)
	}

	for _, d := range []string{"cl:auto", "cl:h:48", "cl:h:0", "cl:glob", "cl:absent", "cl:abspg:0", "cl:del:de1", "cl:dok:de1"} {
		txt, kb := tb.handleCleanCallback(1, d)
		check("clean "+d, txt, kb)
	}

	if mainMenuKB() == nil || updateKB() == nil || backKB() == nil ||
		tb.visKB(1) == nil || tb.subKB() == nil || tb.settingsKB() == nil ||
		tb.webKB() == nil || pingKB() == nil || summaryKB() == nil {
		t.Error("конструктор клавиатуры вернул nil")
	}
}

func itoa64(n int64) string { return itoa(int(n)) }
