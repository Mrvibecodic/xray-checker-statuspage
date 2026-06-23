// Package sub фильтрует подписку, которую наш сервис отдаёт xray-checker по /sub:
// убирает строки серверов, выключенных через бота (servers_meta.enabled=0).
// Поддерживает base64-обёрнутые подписки и простые списки строк; ремарку берёт
// из #fragment (url-декодирует). Это «прослойка» из ПЛАН §8.2.
package sub

import (
	"encoding/base64"
	"net/url"
	"strings"
	"unicode/utf8"
)

// Filter возвращает подписку без серверов, чьи имена (ремарки) есть в disabled.
// Кодировка результата совпадает со входной (base64 → base64).
func Filter(raw []byte, disabled map[string]bool) []byte {
	text := strings.TrimSpace(string(raw))
	wasB64, decoded := tryBase64(text)
	body := text
	if wasB64 {
		body = decoded
	}
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	keep := make([]string, 0, len(lines))
	for _, line := range lines {
		l := strings.TrimSpace(line)
		if l == "" {
			continue
		}
		name := Remark(l)
		if name != "" && disabled[name] {
			continue
		}
		keep = append(keep, l)
	}
	out := strings.Join(keep, "\n")
	if wasB64 {
		return []byte(base64.StdEncoding.EncodeToString([]byte(out)))
	}
	return []byte(out)
}

// Remark извлекает имя сервера из proxy-URI (часть после последнего '#'),
// url-декодируя её. Пустая строка — если ремарки нет.
func Remark(line string) string {
	i := strings.LastIndex(line, "#")
	if i < 0 || i+1 >= len(line) {
		return ""
	}
	frag := line[i+1:]
	if dec, err := url.QueryUnescape(frag); err == nil {
		frag = dec
	}
	return strings.TrimSpace(frag)
}

// Names возвращает список имён (ремарок) всех серверов в подписке — для
// сопоставления с servers_meta.
func Names(raw []byte) []string {
	text := strings.TrimSpace(string(raw))
	if ok, dec := tryBase64(text); ok {
		text = dec
	}
	var out []string
	seen := map[string]bool{}
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if n := Remark(strings.TrimSpace(line)); n != "" && !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	return out
}

// tryBase64 определяет, является ли вход base64-подпиской (декодируется в текст
// с proxy-URI), и возвращает декодированное тело.
func tryBase64(text string) (bool, string) {
	t := strings.TrimSpace(text)
	if t == "" || strings.Contains(t, "://") {
		return false, "" // уже похоже на plaintext-список
	}
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if dec, err := enc.DecodeString(t); err == nil && utf8.Valid(dec) && strings.Contains(string(dec), "://") {
			return true, string(dec)
		}
	}
	return false, ""
}
