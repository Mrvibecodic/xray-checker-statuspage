// Package bot — Telegram control plane (ПЛАН §7). В M2: авторизация по whitelist
// chat_id (чужие игнорируются молча), read-команды и доставка алертов поллера в
// ЛС админам. Мутации/инциденты/контроль добавляются в M3+.
package bot

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"xray-status/internal/config"
	"xray-status/internal/poller"
	"xray-status/internal/store"
	"xray-status/internal/sub"
	"xray-status/internal/updater"
)

// alertTarget — получатель широковещательных уведомлений: чат + опц. топик
// форума (threadID 0 = обычный чат/General).
type alertTarget struct {
	chatID   int64
	threadID int
}

type Bot struct {
	b            *bot.Bot
	st           *store.Store
	cfg          config.Config
	admins       map[int64]bool
	alertTargets []alertTarget
	upd          *updater.Updater
	web          webStarter

	mu       sync.Mutex
	lastDown map[string]bool // name -> последнее отправленное состояние (true=down)
	pollNow  func(context.Context) error
}

// startRefresh обновляет статус НЕ блокируя кнопку: правит сообщение на
// «обновляю…», опрашивает чекер в фоне, затем показывает статус-панель.
func (tb *Bot) startRefresh(ctx context.Context, chatID int64, msgID int) {
	tb.editMessage(ctx, chatID, msgID, "🔄 Обновляю…", nil)
	go func() {
		bg := context.Background()
		tb.refreshNow(bg)
		tb.editMessage(bg, chatID, msgID, tb.mainMenuText(), mainMenuKB())
	}()
}

// SetPollNow даёт боту возможность дёрнуть немедленный опрос поллера.
func (tb *Bot) SetPollNow(f func(context.Context) error) { tb.pollNow = f }

// refreshNow синхронно опрашивает чекер, чтобы данные были свежими сразу.
func (tb *Bot) refreshNow(ctx context.Context) {
	if tb.pollNow == nil {
		return
	}
	c, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	_ = tb.pollNow(c)
}

// New создаёт бота. Если токен не задан — возвращает (nil, nil): бот выключен.
func New(cfg config.Config, st *store.Store) (*Bot, error) {
	if cfg.BotToken == "" {
		return nil, nil
	}
	admins := map[int64]bool{}
	for _, id := range cfg.BotAdminIDs {
		admins[id] = true
	}
	// alertTargets — получатели алертов/сводок: админы + доп. чаты/группы из
	// NOTIFY_CHAT_IDS. Права на управление ботом (admins) сюда НЕ распространяются.
	alertTargets := make([]alertTarget, 0, len(admins)+len(cfg.NotifyTargets))
	seenTarget := map[string]bool{}
	addTarget := func(chatID int64, threadID int) {
		key := strconv.FormatInt(chatID, 10) + ":" + strconv.Itoa(threadID)
		if seenTarget[key] {
			return
		}
		seenTarget[key] = true
		alertTargets = append(alertTargets, alertTarget{chatID: chatID, threadID: threadID})
	}
	for id := range admins {
		addTarget(id, 0)
	}
	for _, t := range cfg.NotifyTargets {
		addTarget(t.ChatID, t.ThreadID)
	}
	tb := &Bot{st: st, cfg: cfg, admins: admins, alertTargets: alertTargets, lastDown: map[string]bool{},
		upd: updater.New(cfg.UpdateURL, cfg.UpdateSHA256URL)}
	b, err := bot.New(cfg.BotToken, bot.WithDefaultHandler(tb.onUpdate))
	if err != nil {
		return nil, err
	}
	tb.b = b
	return tb, nil
}

// Run запускает long-polling до отмены контекста.
func (tb *Bot) Run(ctx context.Context) {
	if tb == nil || tb.b == nil {
		return
	}
	slog.Info("telegram bot started", "admins", len(tb.admins))
	if len(tb.admins) == 0 {
		slog.Warn("BOT_ADMIN_IDS пуст — бот не примет ни одной команды; впиши свой chat_id (@userinfobot)")
	}
	if _, err := tb.b.SetMyCommands(ctx, &bot.SetMyCommandsParams{Commands: botCommands()}); err != nil {
		slog.Warn("set commands", "err", err)
	}
	// подтверждение после перезапуска/обновления — РЕДАКТИРУЕМ то же сообщение
	// (никаких новых «висящих» сообщений).
	if v := tb.st.GetBotState(0, "restart_notify"); v != "" {
		if cid, mid, ok := parseNotify(v); ok {
			tb.editMessage(ctx, cid, mid, "✅ Готово. Сервис перезапущен.\n\n"+tb.mainMenuText(), mainMenuKB())
		}
		_ = tb.st.DelBotState(0, "restart_notify")
	}
	tb.b.Start(ctx)
}

// AttachWeb подключает контроллер веб-сервера (старт/стоп по кнопке).
func (tb *Bot) AttachWeb(w webStarter) { tb.web = w }

func (tb *Bot) onUpdate(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.CallbackQuery != nil {
		tb.onCallback(ctx, update)
		return
	}
	if update.Message == nil {
		return
	}
	chatID := update.Message.Chat.ID
	// Чужих в публичной выдаче игнорируем (стелс), но логируем их chat_id —
	// чтобы при первой настройке было видно, какой id вписать в BOT_ADMIN_IDS.
	if !tb.admins[chatID] {
		slog.Info("сообщение от не-админа (впиши этот chat_id в BOT_ADMIN_IDS, если это ты)",
			"chat_id", chatID)
		return
	}
	// Режим ожидания ввода значения (домен и т.п.) — кнопочный FSM.
	if await := tb.st.GetBotState(chatID, "await"); await != "" {
		if await == "favicon" {
			tb.handleFaviconUpload(ctx, chatID, update.Message)
			return
		}
		txt := strings.TrimSpace(update.Message.Text)
		if !strings.HasPrefix(txt, "/") {
			tb.handleAwait(ctx, chatID, await, txt, update.Message.ID)
			return
		}
		tb.st.DelBotState(chatID, "await")
		tb.st.DelBotState(chatID, "await_msg")
	}

	// Команду/сообщение админа удаляем сразу — чат остаётся чистым, в нём живут
	// только кнопочные сообщения бота (в ЛС бот вправе удалять входящие).
	_, _ = tb.b.DeleteMessage(ctx, &bot.DeleteMessageParams{ChatID: chatID, MessageID: update.Message.ID})

	fields := strings.Fields(strings.TrimSpace(update.Message.Text))
	if len(fields) == 0 {
		return
	}
	cmd := fields[0]
	if i := strings.IndexByte(cmd, '@'); i >= 0 { // /status@MyBot
		cmd = cmd[:i]
	}

	var text string
	switch cmd {
	case "/start", "/menu":
		tb.sendMenu(ctx, chatID)
		return
	case "/help":
		text = helpText()
	case "/status":
		text = statusText(tb.st, tb.cfg)
	case "/servers":
		text = serversText(tb.st, tb.cfg)
	case "/stats":
		text = statsText(tb.st, tb.cfg)
	case "/incident":
		text = cmdIncident(tb.st, fields[1:], chatID)
	case "/incidents":
		tb.sendSection(ctx, chatID, tb.incidentsText(), tb.incidentsKB())
		return
	case "/maintenance":
		tb.sendSection(ctx, chatID, maintText(tb.st, tb.cfg), maintKB(tb.st, tb.cfg))
		return
	case "/settings":
		tb.sendSection(ctx, chatID, tb.settingsText(), tb.settingsKB())
		return
	case "/set":
		text = cmdSet(tb.st, fields[1:])
	case "/nginx", "/domain":
		text = cmdNginx(tb.st, tb.cfg)
	case "/update":
		tb.sendSection(ctx, chatID, tb.updateText(), updateKB())
		return
	case "/sub":
		tb.sendSection(ctx, chatID, tb.subText(), tb.subKB())
		return
	case "/server":
		tb.sendSection(ctx, chatID, tb.visText(), tb.visKB(chatID))
		return
	case "/audit":
		text = cmdAudit(tb)
	default:
		tb.sendMenu(ctx, chatID)
		return
	}
	tb.reply(ctx, chatID, text)
}

func (tb *Bot) reply(ctx context.Context, chatID int64, text string) {
	tb.replyTo(ctx, chatID, 0, text)
}

// replyTo шлёт текст в чат, опционально в топик форума (threadID>0).
func (tb *Bot) replyTo(ctx context.Context, chatID int64, threadID int, text string) {
	p := &bot.SendMessageParams{ChatID: chatID, Text: text, ParseMode: models.ParseModeHTML}
	if threadID > 0 {
		p.MessageThreadID = threadID
	}
	if _, err := tb.b.SendMessage(ctx, p); err != nil {
		slog.Error("telegram send", "err", err)
	}
}

// sendMenu отправляет главное меню с инлайн-кнопками.
func (tb *Bot) sendMenu(ctx context.Context, chatID int64) {
	if _, err := tb.b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        tb.mainMenuText(),
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: mainMenuKB(),
	}); err != nil {
		slog.Error("telegram send menu", "err", err)
	}
}

// onCallback навигирует по меню, РЕДАКТИРУЯ то же сообщение (а не плодя новые).
func (tb *Bot) onCallback(ctx context.Context, update *models.Update) {
	cq := update.CallbackQuery
	if cq == nil {
		return
	}
	if !tb.admins[cq.From.ID] {
		tb.answer(ctx, cq.ID)
		return
	}
	var chatID int64
	var msgID int
	if cq.Message.Message != nil {
		chatID = cq.Message.Message.Chat.ID
		msgID = cq.Message.Message.ID
	}

	// любая навигация по кнопкам отменяет режим ожидания ввода (если он был),
	// чтобы следующее текстовое сообщение не «засосалось» как значение.
	tb.st.DelBotState(cq.From.ID, "await")
	tb.st.DelBotState(cq.From.ID, "await_msg")

	// служебная кнопка-счётчик пагинации — просто гасим «часики»
	if cq.Data == "noop" {
		tb.answer(ctx, cq.ID)
		return
	}

	// действия, управляющие своим сообщением (без мусора):
	switch cq.Data {
	case "m:doupdate":
		tb.startUpdate(ctx, chatID, msgID)
		tb.answer(ctx, cq.ID)
		return
	case "m:restart":
		tb.startRestart(ctx, chatID, msgID)
		tb.answer(ctx, cq.ID)
		return
	case "m:webstart", "m:webstop":
		tb.startWebAction(ctx, chatID, msgID, cq.Data)
		tb.answer(ctx, cq.ID)
		return
	case "m:refresh":
		tb.startRefresh(ctx, chatID, msgID)
		tb.answer(ctx, cq.ID)
		return
	case "sub:diag":
		tb.startSubDiag(ctx, chatID, msgID)
		tb.answer(ctx, cq.ID)
		return
	}

	var text string
	kb := backKB()
	switch {
	case cq.Data == "m:upcheck":
		text = tb.checkUpdateText(ctx)
		kb = updateKB()
	case strings.HasPrefix(cq.Data, "set:"):
		text, kb = tb.handleSettingCallback(cq.From.ID, msgID, cq.Data)
	case strings.HasPrefix(cq.Data, "mnt:"):
		text, kb = tb.handleMaintCallback(cq.From.ID, cq.Data)
	case strings.HasPrefix(cq.Data, "inc:"):
		text, kb = tb.handleIncCallback(cq.From.ID, msgID, cq.Data)
	case strings.HasPrefix(cq.Data, "sub:"):
		text, kb = tb.handleSubCallback(cq.From.ID, msgID, cq.Data)
	case strings.HasPrefix(cq.Data, "vis:"):
		text, kb = tb.handleVisCallback(cq.From.ID, cq.Data)
	case strings.HasPrefix(cq.Data, "mute:"):
		text, kb = tb.handleMuteCallback(cq.From.ID, cq.Data)
	case strings.HasPrefix(cq.Data, "cl:"):
		text, kb = tb.handleCleanCallback(cq.From.ID, cq.Data)
	default:
		text, kb = tb.sectionText(cq.From.ID, cq.Data)
	}
	if msgID != 0 {
		tb.editMessage(ctx, chatID, msgID, text, kb)
	}
	tb.answer(ctx, cq.ID)
}

func (tb *Bot) answer(ctx context.Context, id string) {
	_, _ = tb.b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{CallbackQueryID: id})
}

func (tb *Bot) sendSection(ctx context.Context, chatID int64, text string, kb *models.InlineKeyboardMarkup) {
	p := &bot.SendMessageParams{ChatID: chatID, Text: text, ParseMode: models.ParseModeHTML}
	if kb != nil {
		p.ReplyMarkup = kb
	}
	if _, err := tb.b.SendMessage(ctx, p); err != nil {
		slog.Error("telegram send", "err", err)
	}
}

// HandleEvent доставляет алерт поллера в ЛС админам (реализует notify.Notifier).
// Простейший анти-флап: не дублируем одинаковое состояние сервера подряд.
func (tb *Bot) HandleEvent(e poller.Event) {
	if tb == nil || tb.b == nil {
		return
	}
	if e.Name != "" {
		if muted, _ := tb.st.MutedSet(); muted[e.Name] {
			return // алерты по этому серверу заглушены админом (🔕 Тихие серверы)
		}
	}
	if e.Type == poller.EventServerDown || e.Type == poller.EventServerUp {
		if !tb.st.AlertOnDown() {
			return
		}
		down := e.Type == poller.EventServerDown
		tb.mu.Lock()
		prev, seen := tb.lastDown[e.Name]
		if seen && prev == down {
			tb.mu.Unlock()
			return
		}
		tb.lastDown[e.Name] = down
		tb.mu.Unlock()
	}
	text := eventText(e)
	if text == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, t := range tb.alertTargets {
		tb.replyTo(ctx, t.chatID, t.threadID, text)
	}
}

// RunScheduler шлёт ежедневную сводку админам в заданное время (HH:MM в TZ).
func (tb *Bot) RunScheduler(ctx context.Context) {
	if tb == nil || tb.b == nil {
		return
	}
	loc, err := time.LoadLocation(tb.cfg.TZ)
	if err != nil {
		loc = time.UTC
	}
	lastSent := ""
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now().In(loc)
			if dueSummary(tb.st.DailySummaryTime(), now, lastSent) {
				lastSent = now.Format("2006-01-02")
				text := dailySummaryText(tb.st, tb.cfg)
				sctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				for _, t := range tb.alertTargets {
					tb.replyTo(sctx, t.chatID, t.threadID, text)
				}
				cancel()
			}
		}
	}
}

// dueSummary — пора ли слать сводку: время задано, совпало с текущим HH:MM,
// и сегодня ещё не слали.
func dueSummary(hhmm string, now time.Time, lastSent string) bool {
	if hhmm == "" {
		return false
	}
	return now.Format("15:04") == hhmm && lastSent != now.Format("2006-01-02")
}

// editMessage правит существующее сообщение (текст + клавиатуру).
func (tb *Bot) editMessage(ctx context.Context, chatID int64, msgID int, text string, kb *models.InlineKeyboardMarkup) {
	p := &bot.EditMessageTextParams{
		ChatID:    chatID,
		MessageID: msgID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
	}
	if kb != nil { // типизированный nil в ReplyMarkup даёт "object expected as reply markup"
		p.ReplyMarkup = kb
	}
	if _, err := tb.b.EditMessageText(ctx, p); err != nil {
		slog.Error("telegram edit", "err", err)
	}
}

// handleAwait принимает введённое админом значение, удаляет его сообщение и
// возвращает раздел настроек (правкой исходного сообщения).
func (tb *Bot) handleAwait(ctx context.Context, chatID int64, await, txt string, msgID int) {
	awaitMsg := tb.st.GetBotState(chatID, "await_msg")
	tb.st.DelBotState(chatID, "await")
	tb.st.DelBotState(chatID, "await_msg")
	oldMsg, _ := strconv.Atoi(awaitMsg)

	// удаляем сообщение админа с введёнными данными (без мусора в чате)
	_, _ = tb.b.DeleteMessage(ctx, &bot.DeleteMessageParams{ChatID: chatID, MessageID: msgID})

	// Подписка и домен требуют опроса/перезапуска (это небыстро) — делаем в фоне,
	// бот не виснет, и ведём на рабочую панель, а не на пустую.
	switch await {
	case "sub_add":
		added, addErr := tb.st.AddSubscriptionURLs(sub.ParseURLs(txt))
		_ = tb.st.AddAudit(chatID, "sub_added", itoa(added), "", auditRes(addErr))
		// остаёмся в режиме добавления — можно слать ещё подписки по одной;
		// любая кнопка («Готово»/навигация) снимет ожидание (см. onCallback).
		_ = tb.st.SetBotState(chatID, "await", "sub_add")
		_ = tb.st.SetBotState(chatID, "await_msg", itoa(oldMsg))
		doneKB := &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
			{ikb("✅ Готово", "m:sub")},
		}}
		head := "➕ Добавлено: " + itoa(added) + ". Пришли ещё или нажми «Готово».\n\n" + tb.subText()
		if addErr != nil {
			head = "⚠️ Не удалось сохранить подписку — попробуй ещё раз.\n\n" + tb.subText()
		}
		if oldMsg > 0 {
			tb.editMessage(ctx, chatID, oldMsg, head, doneKB)
		} else {
			tb.sendSection(ctx, chatID, head, doneKB)
		}
		if addErr == nil {
			go func() { tb.refreshNow(context.Background()) }() // опросить чекер в фоне
		}
		return
	case "domain":
		msg := cmdSet(tb.st, []string{"domain", txt})
		// cmdSet возвращает сообщение об успехе только когда домен реально сохранён;
		// при пустом/битом вводе — текст ошибки. Не выдаём провал за успех и не
		// дёргаем рестарт веба зря.
		ok := strings.HasPrefix(msg, "🌐 Домен сохранён")
		if oldMsg > 0 {
			_, _ = tb.b.DeleteMessage(ctx, &bot.DeleteMessageParams{ChatID: chatID, MessageID: oldMsg})
		}
		if !ok {
			tb.sendSection(ctx, chatID, msg, tb.webKB())
			return
		}
		sid := tb.sendReturnID(ctx, chatID, "🌐 Домен сохранён. Перезапускаю веб-сервер…", nil)
		go func() {
			bg := context.Background()
			if tb.web != nil && tb.web.Running() {
				_ = tb.web.Start() // перезапуск с новым доменом — HTTPS поднимется сам
			}
			tb.editOrSend(bg, chatID, sid, tb.webText(), tb.webKB())
		}()
		return
	}

	var text string
	var kb *models.InlineKeyboardMarkup
	switch await {
	case "page_title":
		_ = tb.st.AddAudit(chatID, "title_set", "", txt, auditRes(tb.st.SetSetting("title", txt)))
		text, kb = tb.pageText(), tb.pageKB()
	case "page_subtitle":
		_ = tb.st.AddAudit(chatID, "subtitle_set", "", txt, auditRes(tb.st.SetSetting("subtitle", txt)))
		text, kb = tb.pageText(), tb.pageKB()
	case "page_desc":
		_ = tb.st.AddAudit(chatID, "desc_set", "", txt, auditRes(tb.st.SetSetting("description", txt)))
		text, kb = tb.pageText(), tb.pageKB()
	case "inc_title":
		// заголовок принят → выбор затронутых серверов (инцидент создаётся
		// после «Создать» в incAff). inc_sev уже сохранён.
		_ = tb.st.SetBotState(chatID, "inc_title_txt", txt)
		_ = tb.st.SetBotState(chatID, "inc_aff", "")
		tb.setPage(chatID, "aff_pg", 0)
		text, kb = tb.incAffText(chatID), tb.incAffKB(chatID)
	default:
		tb.sendMenu(ctx, chatID)
		return
	}

	tb.editOrSend(ctx, chatID, oldMsg, text, kb)
}

// sendReturnID отправляет сообщение и возвращает его ID (0 при ошибке).
func (tb *Bot) sendReturnID(ctx context.Context, chatID int64, text string, kb *models.InlineKeyboardMarkup) int {
	p := &bot.SendMessageParams{ChatID: chatID, Text: text, ParseMode: models.ParseModeHTML}
	if kb != nil {
		p.ReplyMarkup = kb
	}
	m, err := tb.b.SendMessage(ctx, p)
	if err != nil || m == nil {
		slog.Error("telegram send", "err", err)
		return 0
	}
	return m.ID
}

// editOrSend правит сообщение по ID, иначе шлёт новое.
func (tb *Bot) editOrSend(ctx context.Context, chatID int64, msgID int, text string, kb *models.InlineKeyboardMarkup) {
	if msgID > 0 {
		tb.editMessage(ctx, chatID, msgID, text, kb)
	} else {
		tb.sendSection(ctx, chatID, text, kb)
	}
}
func botCommands() []models.BotCommand {
	return []models.BotCommand{
		{Command: "menu", Description: "🎛 Главное меню"},
		{Command: "status", Description: "📊 Статус серверов"},
		{Command: "servers", Description: "🖥 Список серверов"},
		{Command: "stats", Description: "📈 Статистика аптайма"},
		{Command: "incidents", Description: "🚨 Инциденты"},
		{Command: "maintenance", Description: "🛠 Обслуживание"},
		{Command: "settings", Description: "⚙️ Настройки и уведомления"},
		{Command: "audit", Description: "📜 Журнал действий"},
		{Command: "update", Description: "⬆️ Обновление сервиса"},
		{Command: "help", Description: "❓ Помощь"},
	}
}

func helpText() string {
	return "<b>xray-status</b>\nВсё управление — кнопками.\nОткрой меню: /menu"
}

// parseNotify разбирает строку "chatID:msgID".
func parseNotify(v string) (int64, int, bool) {
	parts := strings.SplitN(v, ":", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	cid, e1 := strconv.ParseInt(parts[0], 10, 64)
	mid, e2 := strconv.Atoi(parts[1])
	if e1 != nil || e2 != nil || cid == 0 || mid == 0 {
		return 0, 0, false
	}
	return cid, mid, true
}
