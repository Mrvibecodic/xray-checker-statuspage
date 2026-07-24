package sub

import (
	"bytes"
	"encoding/json"
	"strings"
)

// DiagConfig — один конфиг XRAY_JSON-подписки глазами диагностики.
type DiagConfig struct {
	Remarks string
	Nodes   int // прокси-outbound'ы (без freedom/blackhole/dns)
}

// DiagResult — разбор одного ответа панели для «🩺 Диагностики» в боте.
type DiagResult struct {
	JSON    bool         // ответ распознан как XRAY_JSON
	Configs []DiagConfig // для JSON: конфиги и число узлов в каждом
	Links   int          // для base64/plaintext: количество share-ссылок
}

// Diagnose определяет формат ответа панели и его состав. Сетевых вызовов нет:
// на вход — сырое тело ответа, на выход — что в нём (JSON-конфиги с числом
// узлов или просто строки-ссылки). Правила распознавания те же, что у
// FilterJSON и Merge, чтобы диагностика видела подписку глазами /sub.
func Diagnose(raw []byte) DiagResult {
	t := bytes.TrimSpace(raw)
	if len(t) == 0 {
		return DiagResult{}
	}
	var arr []json.RawMessage
	switch t[0] {
	case '[':
		if json.Unmarshal(t, &arr) != nil {
			return DiagResult{}
		}
	case '{':
		// либо конверт {"data":[...]}, либо одиночный конфиг-объект
		var env struct {
			Data []json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(t, &env); err == nil && env.Data != nil {
			arr = env.Data
		} else {
			arr = []json.RawMessage{t}
		}
	default:
		text := string(t)
		if wasB64, dec := tryBase64(text); wasB64 {
			text = dec
		}
		n := 0
		for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
			if strings.TrimSpace(line) != "" {
				n++
			}
		}
		return DiagResult{Links: n}
	}
	res := DiagResult{JSON: true}
	for _, c := range arr {
		var cfg struct {
			Remarks   string `json:"remarks"`
			Outbounds []struct {
				Protocol string `json:"protocol"`
			} `json:"outbounds"`
		}
		if json.Unmarshal(c, &cfg) != nil {
			continue
		}
		n := 0
		for _, o := range cfg.Outbounds {
			switch o.Protocol {
			case "", "freedom", "blackhole", "dns":
			default:
				n++
			}
		}
		res.Configs = append(res.Configs, DiagConfig{Remarks: cfg.Remarks, Nodes: n})
	}
	return res
}
