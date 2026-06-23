package store

import "strconv"

// Ключи операционных настроек (хранятся в таблице settings, меняются из бота).
const (
	kAlertOnDown   = "alert_on_down"
	kPingThreshold = "ping_threshold_ms"
	kDailySummary  = "daily_summary_time"
	kPublicDomain  = "public_domain"
)

// AlertOnDown — слать ли алерт при падении/восстановлении сервера (дефолт да).
func (s *Store) AlertOnDown() bool { return s.GetSetting(kAlertOnDown, "1") == "1" }

func (s *Store) SetAlertOnDown(b bool) error {
	v := "0"
	if b {
		v = "1"
	}
	return s.SetSetting(kAlertOnDown, v)
}

// PingThreshold — порог высокого пинга в мс (0 = выключено).
func (s *Store) PingThreshold() int {
	n, _ := strconv.Atoi(s.GetSetting(kPingThreshold, "0"))
	if n < 0 {
		n = 0
	}
	return n
}

func (s *Store) SetPingThreshold(ms int) error {
	if ms < 0 {
		ms = 0
	}
	return s.SetSetting(kPingThreshold, strconv.Itoa(ms))
}

// DailySummaryTime — время ежедневной сводки "HH:MM" в TZ сервиса ("" = выключено).
func (s *Store) DailySummaryTime() string { return s.GetSetting(kDailySummary, "") }

func (s *Store) SetDailySummaryTime(hhmm string) error { return s.SetSetting(kDailySummary, hhmm) }

// PublicDomain — публичный домен статуспейджа (для подсказки nginx и ссылок).
func (s *Store) PublicDomain() string { return s.GetSetting(kPublicDomain, "") }

func (s *Store) SetPublicDomain(d string) error { return s.SetSetting(kPublicDomain, d) }
