package bot

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-telegram/bot/models"

	"xray-status/internal/checker"
	"xray-status/internal/config"
	"xray-status/internal/store"
)

func TestNameTokStableAndFits(t *testing.T) {
	n := "🇷🇺 Россия · Москва — основной канал 500 Мбит"
	first := nameTok(n)
	if nameTok(n) != first {
		t.Fatal("nameTok не стабилен")
	}
	if nameTok(n+"x") == first {
		t.Fatal("разные имена дали одинаковый токен")
	}
	if got := len(nameTok(n)); got != 16 {
		t.Fatalf("токен = %d символов, ожидалось 16", got)
	}
	for _, p := range []string{"mute:", "vis:", "mnt:srv:", "inc:aff:"} {
		if len(p+nameTok(n)) > 64 {
			t.Fatalf("callback %q не влезает в лимит Telegram 64 байта", p)
		}
	}
}

// longNameBot — бот с одним сервером, чьё имя в callback_data превышает 64 байта
// (старый код молча дропал кнопку по такому серверу).
func longNameBot(t *testing.T) (*Bot, string) {
	t.Helper()
	st, err := store.Open("sqlite", filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	name := "🇷🇺 Россия · Москва — основной резервный канал"
	if len("mute:"+name) <= 64 {
		t.Fatalf("тестовое имя должно давать callback >64 байт, а даёт %d", len("mute:"+name))
	}
	now := time.Now().UTC()
	if _, err := st.PollWrite([]checker.Proxy{
		{StableID: "s1", Name: name, Online: false},
	}, store.PollWriteParams{Now: now.Unix(), Today: now.Format("2006-01-02"),
		PollInterval: 60, CutoffDay: "2000-01-01", SampleRetainDays: 31}); err != nil {
		t.Fatal(err)
	}
	return &Bot{st: st, cfg: config.Config{TZ: "UTC"}, admins: map[int64]bool{1: true}}, name
}

// srvButtonCB достаёт callback серверной кнопки (с префиксом prefix, но не
// навигацией prefix+"pg:").
func srvButtonCB(kb *models.InlineKeyboardMarkup, prefix string) string {
	for _, row := range kb.InlineKeyboard {
		for _, b := range row {
			if strings.HasPrefix(b.CallbackData, prefix) && !strings.HasPrefix(b.CallbackData, prefix+"pg:") {
				return b.CallbackData
			}
		}
	}
	return ""
}

func TestMuteLongNameHasButtonAndToggles(t *testing.T) {
	tb, name := longNameBot(t)
	cb := srvButtonCB(tb.muteKB(1), "mute:")
	if cb == "" {
		t.Fatal("у длинного сервера должна быть кнопка мьюта (регресс 64-байт лимита)")
	}
	if len(cb) > 64 {
		t.Fatalf("callback %d > 64 байт", len(cb))
	}
	if tb.resolveName(cb[len("mute:"):]) != name {
		t.Fatal("токен не резолвится обратно в полное имя")
	}
	tb.handleMuteCallback(1, cb)
	if muted, _ := tb.st.MutedSet(); !muted[name] {
		t.Fatal("после клика сервер должен быть замьючен по полному имени")
	}
}

func TestVisLongNameHasButtonAndToggles(t *testing.T) {
	tb, name := longNameBot(t)
	cb := srvButtonCB(tb.visKB(1), "vis:")
	if cb == "" {
		t.Fatal("у длинного сервера должна быть кнопка видимости")
	}
	tb.handleVisCallback(1, cb)
	if hidden, _ := tb.st.HiddenSet(); !hidden[name] {
		t.Fatal("после клика сервер должен быть скрыт")
	}
}

func TestMaintLongNameHasButton(t *testing.T) {
	tb, name := longNameBot(t)
	cb := srvButtonCB(tb.maintServersKB(1), "mnt:srv:")
	if cb == "" {
		t.Fatal("у длинного сервера должна быть кнопка обслуживания")
	}
	if _, kb := tb.handleMaintCallback(1, cb); kb == nil {
		t.Fatal("после выбора сервера ожидается меню длительности")
	}
	if tb.st.GetBotState(1, "mnt_srv") != name {
		t.Fatal("выбранный сервер должен сохраниться по полному имени")
	}
}
