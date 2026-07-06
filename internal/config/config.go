// Package config загружает bootstrap-конфигурацию из переменных окружения.
//
// Принцип (см. ПЛАН §5): в окружении живёт ТОЛЬКО то, без чего сервис не
// стартует и не аутентифицируется (порт, адрес чекера, токен бота, ID админов,
// ключ шифрования). Всё операционное (тема, тексты, подписка, тумблеры) в
// последующих фазах переедет в БД и будет меняться из бота на лету.
package config

import (
	"os"
	"runtime"
	"strconv"
	"strings"
)

type Config struct {
	// Веб / поллер
	Port            string
	Title           string
	Subtitle        string
	Description     string
	Days            int
	TZ              string
	CheckerURL      string
	PollInterval    int
	ServerHeader    string
	DBPath          string
	Domain          string
	SubscriptionURL string
	DBDriver        string
	DBDSN           string
	InternalPort    string
	HTTPSPort       string
	CertFile        string
	KeyFile         string
	TLSMode         string

	// Ретеншн / поведение поллера (паритет с app.py)
	SampleRetainDays  int
	StaleAfterHours   int
	GlobalOutageRatio float64

	// Bootstrap для будущих фаз (бот/контроль). Парсим заранее, чтобы
	// docker-compose уже был стабильным; в M0/M1 не используются.
	BotToken      string
	BotAdminIDs   []int64
	NotifyTargets []NotifyTarget
	SecretKey     string
	ControlCaps   []string

	UpdateURL       string
	UpdateSHA256URL string
}

func getenv(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func atoi(k string, def int) int {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func atof(k string, def float64) float64 {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			return n
		}
	}
	return def
}

func splitList(k string) []string {
	raw := strings.TrimSpace(os.Getenv(k))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func splitInt64(k string) []int64 {
	var out []int64
	for _, p := range splitList(k) {
		if n, err := strconv.ParseInt(p, 10, 64); err == nil {
			out = append(out, n)
		}
	}
	return out
}

// NotifyTarget — получатель уведомлений: чат/группа/канал и опциональный топик
// форума (ThreadID). ThreadID 0 => обычный чат или General-топик.
type NotifyTarget struct {
	ChatID   int64
	ThreadID int
}

// parseNotifyTargets парсит NOTIFY_CHAT_IDS: элементы вида `-100123` или
// `-100123:42` (chatID:threadID). Разделитель топика — двоеточие; знак минус в
// chatID не мешает, т.к. ищем ПОСЛЕДНЕЕ двоеточие.
func parseNotifyTargets(k string) []NotifyTarget {
	var out []NotifyTarget
	for _, p := range splitList(k) {
		chatStr, thread := p, 0
		if i := strings.LastIndex(p, ":"); i >= 0 {
			if t, err := strconv.Atoi(strings.TrimSpace(p[i+1:])); err == nil {
				thread = t
				chatStr = strings.TrimSpace(p[:i])
			}
		}
		if id, err := strconv.ParseInt(chatStr, 10, 64); err == nil {
			out = append(out, NotifyTarget{ChatID: id, ThreadID: thread})
		}
	}
	return out
}

// Load читает конфигурацию из окружения, подставляя дефолты, совместимые с
// текущей Python-версией (app.py).
func Load() Config {
	// CONTROL_CAPS пуст => управление полностью включено (бот доступен только
	// админам, поэтому ограничивать нечего). См. canControl.
	caps := splitList("CONTROL_CAPS")
	days := atoi("DAYS", 30)
	defUpdateURL := "https://github.com/Mrvibecodic/xray-checker-statuspage/releases/download/go-build/statuspage-linux-" + runtime.GOARCH
	return Config{
		Port:            getenv("PORT", "80"),
		Title:           getenv("TITLE", "Статус серверов"),
		Subtitle:        getenv("SUBTITLE", "Доступность серверов в реальном времени"),
		Description:     getenv("DESCRIPTION", ""),
		Days:            days,
		TZ:              getenv("TZ", "Europe/Moscow"),
		CheckerURL:      strings.TrimRight(getenv("CHECKER_URL", "http://xray-checker:2112"), "/"),
		PollInterval:    atoi("POLL_INTERVAL", 300),
		ServerHeader:    getenv("SERVER_HEADER", "nginx"),
		Domain:          getenv("DOMAIN", ""),
		SubscriptionURL: getenv("SUBSCRIPTION_URL", ""),
		DBPath:          getenv("DB_PATH", "/data/status.db"),
		DBDriver:        getenv("DB_DRIVER", "sqlite"),
		DBDSN:           getenv("DB_DSN", ""),
		InternalPort:    getenv("INTERNAL_PORT", "8081"),
		HTTPSPort:       getenv("HTTPS_PORT", "443"),
		CertFile:        getenv("CERT_FILE", ""),
		KeyFile:         getenv("KEY_FILE", ""),
		TLSMode:         getenv("TLS_MODE", ""),

		SampleRetainDays:  atoi("SAMPLE_RETAIN_DAYS", days+1),
		StaleAfterHours:   atoi("STALE_AFTER_HOURS", 0),
		GlobalOutageRatio: atof("GLOBAL_OUTAGE_RATIO", 1.0),

		BotToken:      getenv("BOT_TOKEN", ""),
		BotAdminIDs:   splitInt64("BOT_ADMIN_IDS"),
		NotifyTargets: parseNotifyTargets("NOTIFY_CHAT_IDS"),
		SecretKey:     getenv("SECRET_KEY", ""),
		ControlCaps:   caps,

		UpdateURL:       getenv("UPDATE_URL", defUpdateURL),
		UpdateSHA256URL: getenv("UPDATE_SHA256_URL", defUpdateURL+".sha256"),
	}
}
