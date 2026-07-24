package bot

import (
	"context"
	"html"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-telegram/bot/models"

	"xray-status/internal/checker"
	"xray-status/internal/sub"
)

// startSubDiag — кнопка «🩺 Диагностика» в разделе «Подписка»: проверяет цепочку
// панель→/sub→чекер тем же способом, каким это делается руками через curl, и
// одним сообщением говорит, где рвётся. Кнопка не блокируется: сообщение
// редактируется по готовности.
func (tb *Bot) startSubDiag(ctx context.Context, chatID int64, msgID int) {
	tb.editMessage(ctx, chatID, msgID, "🩺 Проверяю подписку и чекер…", nil)
	go func() {
		bg, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		text := tb.subDiagText(bg)
		tb.editMessage(context.Background(), chatID, msgID, text, subDiagKB())
	}()
}

func subDiagKB() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{ikb("🔁 Повторить", "sub:diag"), ikb("◀ Подписка", "m:sub")},
	}}
}

// subDiagText собирает отчёт: формат каждой подписки под UA json-режима чекера
// (Happ/1.0 + X-Hwid), балансир-группы в подписке, что реально видит чекер, и
// вердикт. Секреты не светятся: ссылки маскируются до хоста, как везде в боте.
func (tb *Bot) subDiagText(ctx context.Context) string {
	urls := tb.st.SubscriptionURLs()
	if len(urls) == 0 && tb.cfg.SubscriptionURL != "" {
		urls = sub.ParseURLs(tb.cfg.SubscriptionURL)
	}
	s := "<b>🩺 Диагностика подписки</b>\n"
	if len(urls) == 0 {
		return s + "\nПодписки не заданы — добавь их в разделе «Подписка»."
	}

	client := &http.Client{Timeout: 12 * time.Second}
	jsonSubs, plainSubs, failSubs, subGroups := 0, 0, 0, 0
	s += "\n<b>Подписка (запрос как у чекера в json-режиме):</b>\n"
	for i, u := range urls {
		prefix := itoa(i+1) + ". " + maskURL(u) + " — "
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			failSubs++
			s += prefix + "битая ссылка\n"
			continue
		}
		req.Header.Set("User-Agent", "Happ/1.0")
		req.Header.Set("X-Hwid", "xray-status-diag")
		resp, err := client.Do(req)
		if err != nil {
			failSubs++
			s += prefix + "недоступна\n"
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			failSubs++
			s += prefix + "HTTP " + itoa(resp.StatusCode) + "\n"
			continue
		}
		d := sub.Diagnose(body)
		if !d.JSON {
			plainSubs++
			s += prefix + "ссылки/base64, серверов: " + itoa(d.Links) + " — панель НЕ отдала JSON\n"
			continue
		}
		jsonSubs++
		var groups []string
		for _, c := range d.Configs {
			if c.Nodes > 1 {
				subGroups++
				groups = append(groups, "«"+html.EscapeString(sub.StripTag(c.Remarks))+"» ("+itoa(c.Nodes)+" узл.)")
			}
		}
		s += prefix + "JSON, конфигов: " + itoa(len(d.Configs))
		if len(groups) > 0 {
			s += ", группы: " + strings.Join(groups, ", ")
		}
		s += "\n"
	}

	s += "\n<b>Чекер:</b>\n"
	chkGroups := map[string]int{}
	total, online := 0, 0
	proxies, chkErr := checker.New(tb.cfg.CheckerURL).Fetch(ctx)
	if chkErr != nil {
		s += "недоступен (" + html.EscapeString(chkErr.Error()) + ")\n"
	} else {
		for _, p := range proxies {
			total++
			if p.Online {
				online++
			}
			if p.GroupName != "" {
				chkGroups[p.GroupName]++
			}
		}
		s += "прокси: " + itoa(total) + ", онлайн: " + itoa(online)
		if len(chkGroups) == 0 {
			s += ", балансир-групп нет\n"
		} else {
			var parts []string
			for g, n := range chkGroups {
				parts = append(parts, "«"+html.EscapeString(sub.StripTag(g))+"» ("+itoa(n)+" узл.)")
			}
			s += ", группы: " + strings.Join(parts, ", ") + "\n"
		}
	}

	s += "\n<b>Вердикт:</b>\n"
	switch {
	case failSubs == len(urls):
		s += "🔴 Ни одна подписка не ответила — чекеру нечего проверять. Проверь ссылки и доступность панели."
	case chkErr != nil:
		s += "🔴 Чекер недоступен — проверь CHECKER_URL и контейнер xray-checker."
	case subGroups > 0 && len(chkGroups) == 0:
		s += "⚠ Подписка отдаёт балансир-группы, а чекер их не видит. Обнови xray-checker до ≥1.3.0, включи <code>SUBSCRIPTION_JSON_FORMAT=true</code> и перезапусти его."
	case jsonSubs == 0:
		s += "⚠ Панель не отдала JSON даже под Happ — балансир придёт одной мёртвой ссылкой на фиктивный адрес обёртки. Это настройка панели (шаблон Xray JSON у хоста-обёртки; Remnawave 2.6.3+), статуспейдж на это повлиять не может."
	case plainSubs > 0:
		s += "⚠ Подписки в разных форматах: не-JSON источники не попадают в JSON-выдачу /sub — их серверы чекер не увидит."
	case subGroups == 0:
		s += "ℹ JSON приходит, но балансир-групп в подписке нет. Если группа ожидается — панель не заинжектила узлы в конфиг обёртки (шаблон injectHosts, теги и «Скрыть хост» у бэкендов)."
	default:
		s += "✅ Цепочка в порядке: группы из подписки доходят до чекера."
	}
	return s
}
