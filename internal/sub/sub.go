// Package sub объединяет одну или несколько upstream-подписок в единый формат,
// который принимает xray-checker: декодирует base64/plaintext-списки share-ссылок
// и XRAY_JSON (Remnawave под Happ/1.0), склеивает их в один поток и разводит
// дубли ремарок стабильным тегом. Это «прослойка» из ПЛАН §8.2.
package sub

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
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
// (StripTag), бот работает по полному имени с тегом.
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

// Merge объединяет несколько подписок (каждая base64 или plaintext) в одну
// ленту: декодирует каждую, склеивает строки, убирает дубли строк и разводит
// повторяющиеся ремарки стабильным тегом, затем кодирует результат в base64 —
// единый формат, который xray-checker принимает независимо от формата исходных
// подписок.
func Merge(raws [][]byte) []byte {
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
	// 2) Развести дубли ремарок стабильным тегом.
	keep := make([]string, 0, len(items))
	for _, it := range items {
		name, line := it.remark, it.line
		if name != "" && remarkCount[name] > 1 {
			name = name + tagMark + lineTag(it.line)
			line = setRemark(it.line, name)
		}
		keep = append(keep, line)
	}
	out := strings.Join(keep, "\n")
	return []byte(base64.StdEncoding.EncodeToString([]byte(out)))
}

// FilterJSON обрабатывает подписку в формате XRAY_JSON (массив конфигов, который
// Remnawave отдаёт под UA Happ/1.0): склеивает конфиги из всех переданных
// подписок в один JSON-массив, сохраняя routing/balancers как есть — без этого
// балансеры до чекера не доходят. Второе значение — была ли это вообще
// JSON-подписка; если нет, вызывающий идёт по base64-пути (Merge). Третье —
// сколько НЕ-JSON подписок пришлось отбросить при JSON-выдаче (смешанные
// форматы в один ответ не склеить; вызывающий должен об этом сообщить в лог,
// а не терять серверы молча).
func FilterJSON(raws [][]byte) ([]byte, bool, int) {
	var all []json.RawMessage
	sawJSON := false
	skipped := 0
	for _, raw := range raws {
		t := bytes.TrimSpace(raw)
		if len(t) == 0 {
			continue
		}
		var arr []json.RawMessage
		switch t[0] {
		case '[':
			if err := json.Unmarshal(t, &arr); err != nil {
				skipped++
				continue
			}
		case '{':
			// либо конверт {"data":[...]}, либо одиночный конфиг-объект
			var env struct {
				Data []json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal(t, &env); err == nil && env.Data != nil {
				arr = env.Data
			} else {
				arr = []json.RawMessage{append([]byte(nil), t...)}
			}
		default:
			skipped++
			continue
		}
		sawJSON = true
		all = append(all, arr...)
	}
	if !sawJSON {
		return nil, false, 0
	}
	if all == nil {
		all = []json.RawMessage{}
	}
	out, err := json.Marshal(all)
	if err != nil {
		return nil, false, 0
	}
	return out, true, skipped
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
