// Package sub фильтрует подписку, которую наш сервис отдаёт xray-checker по /sub:
// убирает строки серверов, выключенных через бота (servers_meta.enabled=0).
// Поддерживает base64-обёрнутые подписки и простые списки строк; ремарку берёт
// из #fragment (url-декодирует). Это «прослойка» из ПЛАН §8.2.
package sub

import (
	"encoding/base64"
	"fmt"
	"hash/fnv"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"
)

// tagMark — разделитель тега разведения дублей: когда одна и та же ремарка
// встречается в нескольких подписках, к каждой добавляется стабильный короткий
// тег (хеш строки), чтобы серверы стали различимы. Страница тег срезает
// (StripTag), бот и фильтрация подписки работают по полному имени с тегом.
const tagMark = " ·#"

var tagRe = regexp.MustCompile(` ·#[0-9a-f]{1,8}$`)

// StripTag убирает тег разведения дублей для публичного отображения.
func StripTag(name string) string { return tagRe.ReplaceAllString(name, "") }

// lineTag — стабильный короткий хекс-тег для строки прокси (одинаков при каждом
// слиянии, пока строка та же).
func lineTag(line string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(line))
	return fmt.Sprintf("%04x", h.Sum32()&0xffff)
}

// setRemark заменяет #fragment строки на percent-encoded имя (совместимо с
// Remark: пробел→%20, не-ASCII и '#' экранируются).
func setRemark(line, name string) string {
	i := strings.LastIndex(line, "#")
	if i < 0 {
		return line + "#" + url.PathEscape(name)
	}
	return line[:i+1] + url.PathEscape(name)
}

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

// Merge объединяет несколько подписок (каждая base64 или plaintext) в одну
// ленту: декодирует каждую, склеивает строки, убирает дубли и выключенные
// серверы, и кодирует результат в base64 — единый формат, который xray-checker
// принимает независимо от формата исходных подписок.
func Merge(raws [][]byte, disabled map[string]bool) []byte {
	// 1) Собрать уникальные строки (по полной строке), сохраняя порядок, и
	// посчитать, сколько раз встречается каждая ремарка.
	type item struct{ line, remark string }
	var items []item
	seen := map[string]bool{}
	remarkCount := map[string]int{}
	for _, raw := range raws {
		text := strings.TrimSpace(string(raw))
		if wasB64, dec := tryBase64(text); wasB64 {
			text = dec
		}
		for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
			l := strings.TrimSpace(line)
			if l == "" || seen[l] {
				continue
			}
			seen[l] = true
			r := Remark(l)
			items = append(items, item{l, r})
			if r != "" {
				remarkCount[r]++
			}
		}
	}
	// 2) Развести дубли ремарок стабильным тегом и отфильтровать выключенные
	// (проверка по эффективному — уже с тегом — имени).
	keep := make([]string, 0, len(items))
	for _, it := range items {
		name, line := it.remark, it.line
		if name != "" && remarkCount[name] > 1 {
			name = name + tagMark + lineTag(it.line)
			line = setRemark(it.line, name)
		}
		if name != "" && disabled[name] {
			continue
		}
		keep = append(keep, line)
	}
	out := strings.Join(keep, "\n")
	return []byte(base64.StdEncoding.EncodeToString([]byte(out)))
}

// ParseURLs разбивает пользовательский ввод в список URL подписок. Допускает
// разделители: запятую, перенос строки и пробел — чтобы можно было прислать
// список через запятую, по строке на ссылку или каждую отдельным сообщением.
func ParseURLs(text string) []string {
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == ' ' || r == '\t'
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
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
