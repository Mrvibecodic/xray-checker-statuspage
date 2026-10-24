// Package web — словарь интерфейса.
//
// Все видимые пользователю строки шаблонов вынесены в __T_*__-плейсхолдеры и
// подставляются ПО-ЗАПРОСНО после анти-фингерпринта (uniquify): один шаблон на
// тему обслуживает обе локали (ru/en). Локаль фиксирована стартом процесса
// (LOCALE), поэтому strings.Replacer собирается один раз в New().
package web

import (
	"strings"

	"xray-status/internal/config"
)

// langCode — "en" для английских локалей, иначе "ru".
func langCode(locale string) string {
	if config.IsEnglish(locale) {
		return "en"
	}
	return "ru"
}

// dict — видимые строки интерфейса по локалям. Ключ — имя плейсхолдера
// __T_<KEY>__ в шаблонах.
func dict(locale string) map[string]string {
	if config.IsEnglish(locale) {
		return map[string]string{
			"LOADING":        "Loading…",
			"LOADING_DATA":   "Loading data…",
			"SERVERS_ONLINE": "Servers online",
			"ACTIVE_SERVERS": "Active servers",
			"UPTIME_FOR":     "Uptime over",
			"DAY":            "d",
			"AVG_PING":       "Average ping",
			"LAST_CHECK":     "Last check",
			"SUMMARY":        "Summary",
			"SERVERS":        "Servers",
			"SERVERS_DESC2":  "30-day availability history, current latency, click for details.",
			"SERVERS_DESC3":  "30-day server availability history.",
			"LEG_OK":         "ok",
			"LEG_30":         "up to 30 min",
			"LEG_2H":         "30 min – 2 h",
			"LEG_DOWN":       "over 2 h",
			"NODATA":         "no data",
			"RANGE":          "days ago · today",
			"TODAY":          "Today",
			"MS":             "ms",
			"MIN":            "min",
			"HOUR":           "h",
			"MAINT":          "under maintenance",
			"UNTIL":          " · until ",
			"DOWNTIME":       "downtime: ",
			"BRIEF":          "brief outage",
			"NOOUT":          "no outages",
			"UPTIME":         "uptime: ",
			"UP30":           "uptime 30 d: ",
			"STATUS":         "status: ",
			"ONLINE":         "online",
			"OFFLINE":        "offline",
			"TOTALDOWN":      "total downtime: ",
			"NODAYDATA":      "No data for this day.",
			"CHECKS_TODAY":   "Checks today",
			"CHECKS_DAY":     "Checks that day",
			"POLLERRS":       "Poll errors",
			"PING_NOW":       "Ping now",
			"PING_LAST":      "Last ping",
			"PING_STATS":     "Ping min / avg / max",
			"PINGCAP":        "Ping, ms · log scale · max ",
			"CHARTHINT":      " · blue line — ping, red bars — outage periods",
			"POLL":           "poll: ",
			"SUCCESS":        "success",
			"PING":           "ping: ",
			"POLLERR1":       "poll error",
			"ALL_OK":         "All systems operational",
			"ALL_OK2":        "All active servers are reachable",
			"UNAVAIL":        " server(s) unreachable",
			"LASTCHECK":      "last check ",
			"WAITFIRST":      "waiting for first check",
			"UPDATEEVERY":    " · updates every ",
			"LOADFAIL":       "Failed to load.",
			"MAINTSHORT":     "maint.",
			"UNTIL_APPROX":   "≈ until ",
			"INPROGRESS":     "in progress",
			"NODES":          " nodes",
			"SEV_MINOR":      "Minor",
			"SEV_MAJOR":      "Major",
			"SEV_CRIT":       "Critical",
			"ST_INV":         "Investigating",
			"ST_IDENT":       "Identified",
			"ST_MON":         "Monitoring",
			"ST_RES":         "Resolved",
			"STARTED":        "🕒 started ",
			"AFFECTED":       "🎯 affected: ",
			"UPDATEFAILED":   "Update failed — retrying…",
		}
	}
	return map[string]string{
		"LOADING":        "Загрузка…",
		"LOADING_DATA":   "Загрузка данных…",
		"SERVERS_ONLINE": "Серверов онлайн",
		"ACTIVE_SERVERS": "Активные серверы",
		"UPTIME_FOR":     "Аптайм за",
		"DAY":            "дн",
		"AVG_PING":       "Средний пинг",
		"LAST_CHECK":     "Последняя проверка",
		"SUMMARY":        "Сводка",
		"SERVERS":        "Серверы",
		"SERVERS_DESC2":  "30-дневная история доступности, текущая задержка и детализация по клику.",
		"SERVERS_DESC3":  "30-дневная история доступности серверов.",
		"LEG_OK":         "норма",
		"LEG_30":         "до 30 мин",
		"LEG_2H":         "30 мин – 2 ч",
		"LEG_DOWN":       "от 2 ч",
		"NODATA":         "нет данных",
		"RANGE":          "дней назад · сегодня",
		"TODAY":          "Сегодня",
		"MS":             "мс",
		"MIN":            "мин",
		"HOUR":           "ч",
		"MAINT":          "на обслуживании",
		"UNTIL":          " · до ",
		"DOWNTIME":       "простой: ",
		"BRIEF":          "кратковременный сбой",
		"NOOUT":          "сбоев нет",
		"UPTIME":         "аптайм: ",
		"UP30":           "аптайм 30 дн: ",
		"STATUS":         "статус: ",
		"ONLINE":         "онлайн",
		"OFFLINE":        "офлайн",
		"TOTALDOWN":      "общий простой: ",
		"NODAYDATA":      "Данных за этот день нет.",
		"CHECKS_TODAY":   "Проверок сегодня",
		"CHECKS_DAY":     "Проверок за день",
		"POLLERRS":       "Ошибок опроса",
		"PING_NOW":       "Пинг сейчас",
		"PING_LAST":      "Последний пинг",
		"PING_STATS":     "Пинг мин / сред / макс",
		"PINGCAP":        "Пинг, мс · логарифмическая шкала · макс ",
		"CHARTHINT":      " · синяя линия — пинг, красные полосы — периоды сбоев",
		"POLL":           "опрос: ",
		"SUCCESS":        "успешно",
		"PING":           "пинг: ",
		"POLLERR1":       "ошибка опроса",
		"ALL_OK":         "Все системы в норме",
		"ALL_OK2":        "Все активные серверы доступны",
		"UNAVAIL":        " сервер(ов) недоступно",
		"LASTCHECK":      "последняя проверка ",
		"WAITFIRST":      "ожидание первой проверки",
		"UPDATEEVERY":    " · обновление раз в ",
		"LOADFAIL":       "Не удалось загрузить.",
		"MAINTSHORT":     "обслуж.",
		"UNTIL_APPROX":   "≈ до ",
		"INPROGRESS":     "идут работы",
		"NODES":          " узлов",
		"SEV_MINOR":      "Незначительный",
		"SEV_MAJOR":      "Серьёзный",
		"SEV_CRIT":       "Критический",
		"ST_INV":         "Расследуем",
		"ST_IDENT":       "Причина найдена",
		"ST_MON":         "Наблюдаем",
		"ST_RES":         "Устранён",
		"STARTED":        "🕒 начат ",
		"AFFECTED":       "🎯 затронуты: ",
		"UPDATEFAILED":   "Обновление не удалось — пробуем снова…",
	}
}

// trReplacer собирает подстановку __LANG__/__T_*__ для локали.
func trReplacer(locale string) *strings.Replacer {
	d := dict(locale)
	pairs := make([]string, 0, 2*len(d)+2)
	pairs = append(pairs, "__LANG__", langCode(locale))
	for k, v := range d {
		pairs = append(pairs, "__T_"+k+"__", v)
	}
	return strings.NewReplacer(pairs...)
}
