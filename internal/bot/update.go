package bot

import (
	"context"
	"os"
	"strconv"
	"time"

	"xray-status/internal/updater"
	"xray-status/internal/version"
)

func (tb *Bot) updateText() string {
	s := "<b>Обновление</b>\nВетка: <code>go-build</code>\nТекущая версия: <code>" +
		htmlEscape(version.Version) + "</code>\n\n"
	if tb.upd != nil && tb.upd.Available() {
		s += "«Проверить» — есть ли новая версия и что в ней.\n" +
			"«Обновить сейчас» — скачать, применить и перезапуститься."
	} else {
		s += "Самообновление недоступно."
	}
	return s
}

func (tb *Bot) checkUpdateText(ctx context.Context) string {
	if tb.upd == nil || !tb.upd.Available() {
		return "Самообновление недоступно."
	}
	cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	res, err := tb.upd.Check(cctx, version.Version)
	if err != nil {
		return "🔍 Не удалось проверить: " + htmlEscape(err.Error())
	}
	if res.HasUpdate {
		return "🆕 Доступно обновление: <code>" + htmlEscape(res.Latest) + "</code>\n" +
			htmlEscape(res.Message) + "\n\nТекущая: <code>" + htmlEscape(version.Version) +
			"</code>\nНажми «Обновить сейчас»."
	}
	return "✅ Установлена последняя версия: <code>" + htmlEscape(version.Version) + "</code>"
}

// startUpdate качает и ставит обновление, редактируя ОДНО сообщение (без мусора).
func (tb *Bot) startUpdate(ctx context.Context, chatID int64, msgID int) {
	if tb.upd == nil || !tb.upd.Available() {
		tb.editMessage(ctx, chatID, msgID, "Самообновление недоступно.", updateKB())
		return
	}
	tb.editMessage(ctx, chatID, msgID, "⏳ Обновление запущено…", nil)
	go func() {
		bg := context.Background()
		time.Sleep(700 * time.Millisecond) // дать тексту отрисоваться в клиенте
		tb.editMessage(bg, chatID, msgID, "⬇️ Скачиваю новую версию…", nil)

		uctx, cancel := context.WithTimeout(bg, 3*time.Minute)
		defer cancel()
		path, err := tb.upd.Apply(uctx)
		if err != nil {
			tb.editMessage(bg, chatID, msgID,
				"❌ Обновление не удалось: "+htmlEscape(err.Error()), updateKB())
			return
		}
		tb.editMessage(bg, chatID, msgID, "✅ Скачано. Применяю и перезапускаюсь…", nil)
		_ = tb.st.SetBotState(0, "restart_notify", strconv.FormatInt(chatID, 10)+":"+strconv.Itoa(msgID))
		time.Sleep(1400 * time.Millisecond)
		tb.reexec(path)
	}()
}

// startRestart перезапускает сервис (то же сообщение, без мусора).
func (tb *Bot) startRestart(ctx context.Context, chatID int64, msgID int) {
	tb.editMessage(ctx, chatID, msgID, "🔁 Перезапускаюсь…", nil)
	_ = tb.st.SetBotState(0, "restart_notify", strconv.FormatInt(chatID, 10)+":"+strconv.Itoa(msgID))
	go func() {
		time.Sleep(800 * time.Millisecond)
		exe, err := os.Executable()
		if err == nil {
			tb.reexec(exe)
		}
		os.Exit(0)
	}()
}

func (tb *Bot) reexec(path string) {
	if err := updater.Reexec(path); err != nil {
		os.Exit(0) // re-exec не вышел — выходим, docker restart поднимет
	}
}
