package bot

import (
	"context"
	"time"

	"github.com/go-telegram/bot/models"
)

// webStarter — рантайм-управление публичным веб-сервером (реализуется webctl).
type webStarter interface {
	Start() error
	Stop() error
	Running() bool
	Probe() bool
	Status() string
}

// effectiveDomain — домен из env DOMAIN или заданный в боте.
func (tb *Bot) effectiveDomain() string {
	if tb.cfg.Domain != "" {
		return tb.cfg.Domain
	}
	return tb.st.PublicDomain()
}

func (tb *Bot) webText() string {
	dom := tb.effectiveDomain()
	running := tb.web != nil && tb.web.Running()
	alive := running && tb.web.Probe()

	s := "<b>🚀 Веб-сервер</b>\n"
	switch {
	case tb.web == nil:
		s += "⚪️ Статус: управление недоступно\n"
	case alive:
		s += "🟢 Статус: <b>работает</b>\n"
	case running:
		s += "🟡 Статус: <b>запускается…</b>\n"
	default:
		s += "🔴 Статус: <b>остановлен</b>\n"
	}

	if dom != "" {
		s += "🔒 Режим: HTTPS · " + tb.tlsModeLabel() + "\n🌐 Домен: <b>" + htmlEscape(dom) + "</b>\n"
		if running {
			s += "🔗 Адрес: https://" + htmlEscape(dom) + "\n"
		}
	} else {
		s += "🌐 Режим: HTTP\n"
	}

	s += "\n"
	switch {
	case tb.web != nil && !running:
		s += "Нажми «🚀 Запустить»."
	case dom == "":
		s += "Чтобы включить HTTPS — задай домен и нажми «🔁 Перезапустить»."
	case running && !alive:
		s += "Сервер только что запущен — обнови раздел через пару секунд.\n" +
			"Если домен за Cloudflare и страница не грузится — поставь SSL/TLS в режим " +
			"<b>Full</b>; для <b>Full (strict)</b> добавь Origin Certificate (CERT_FILE/KEY_FILE)."
	}
	return s
}

func (tb *Bot) tlsModeLabel() string {
	mode := tb.cfg.TLSMode
	if mode == "" {
		if tb.cfg.CertFile != "" && tb.cfg.KeyFile != "" {
			mode = "file"
		} else {
			mode = "selfsigned"
		}
	}
	switch mode {
	case "letsencrypt":
		return "Let's Encrypt (без Cloudflare-прокси)"
	case "file":
		return "свой сертификат (Origin Cert)"
	default:
		return "self-signed (за Cloudflare: SSL=Full)"
	}
}

func (tb *Bot) webKB() *models.InlineKeyboardMarkup {
	var rows [][]models.InlineKeyboardButton
	if tb.web != nil && tb.web.Running() {
		rows = append(rows, []models.InlineKeyboardButton{
			ikb("🔁 Перезапустить", "m:webstart"),
			ikb("⏹ Остановить", "m:webstop"),
		})
	} else {
		rows = append(rows, []models.InlineKeyboardButton{ikb("🚀 Запустить", "m:webstart")})
	}
	rows = append(rows,
		[]models.InlineKeyboardButton{ikb("🌐 Задать домен", "set:domain")},
		[]models.InlineKeyboardButton{ikb("🔧 Конфиг nginx (реверс над ботом)", "m:nginx")},
		[]models.InlineKeyboardButton{ikb("◀ Меню", "m:home")},
	)
	return &models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// startWebAction — запуск/остановка/перезапуск веба со стадиями в тексте (не молча).
func (tb *Bot) startWebAction(ctx context.Context, chatID int64, msgID int, data string) {
	if tb.web == nil {
		tb.editMessage(ctx, chatID, msgID, tb.webText(), tb.webKB())
		return
	}
	doing := "🚀 Запускаю веб-сервер…"
	if data == "m:webstop" {
		doing = "⏹ Останавливаю веб-сервер…"
	} else if tb.web.Running() {
		doing = "🔁 Перезапускаю веб-сервер…"
	}
	tb.editMessage(ctx, chatID, msgID, doing, nil)

	go func() {
		bg := context.Background()
		time.Sleep(600 * time.Millisecond) // дать тексту отрисоваться
		var note string
		if data == "m:webstop" {
			_ = tb.web.Stop()
			note = "⏹ Веб-сервер остановлен.\n\n"
		} else {
			if err := tb.web.Start(); err != nil {
				tb.editMessage(bg, chatID, msgID,
					"❌ Не удалось поднять веб-сервер: "+htmlEscape(err.Error()), tb.webKB())
				return
			}
			note = "✅ Веб-сервер перезапущен.\n\n"
		}
		tb.editMessage(bg, chatID, msgID, note+tb.webText(), tb.webKB())
	}()
}
