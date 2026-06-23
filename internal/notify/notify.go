// Package notify — абстракция доставки уведомлений. Сейчас единственная
// реализация — Telegram-бот (ЛС админам). Интерфейс позволит позже добавить
// публичный канал, не трогая поллер (ПЛАН §7).
package notify

import "xray-status/internal/poller"

type Notifier interface {
	HandleEvent(e poller.Event)
}

// Noop — заглушка, когда бот выключен.
type Noop struct{}

func (Noop) HandleEvent(poller.Event) {}
