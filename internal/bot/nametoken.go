package bot

import (
	"fmt"
	"hash/fnv"

	"xray-status/internal/store"
)

// nameTok — короткий стабильный токен имени сервера для callback_data кнопки.
// Telegram ограничивает callback_data 64 байтами; сырое имя сервера легко
// переполняет лимит (кириллица = 2 байта/символ, флаг-эмодзи = 8 байт), из-за
// чего кнопка молча не добавлялась — сервер пропадал из меню мьюта/видимости/
// обслуживания/инцидента, хотя алерты по нему продолжали идти. Токен — 16
// hex-символов (fnv64a), поэтому "<prefix>:"+токен всегда влезает в лимит, а
// настоящее имя восстанавливается resolveName по актуальному списку серверов.
func nameTok(name string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(name))
	return fmt.Sprintf("%016x", h.Sum64())
}

// groupDisplay — имя, под которым строка current видна всем потребителям
// (страница, алерты, обслуживание): для узла балансировочной группы это имя
// группы (grp), иначе — собственное имя сервера. Меню бота обязаны ключевать
// по нему же: узлы «… | proxy», «… | proxy-2» — это ОДИН сервер «…», и
// скрытие/мьют/обслуживание применяются к группе целиком.
func groupDisplay(r store.CurrentRow) string {
	if r.Grp != "" {
		return r.Grp
	}
	return r.Name
}

// groupNames — список серверов для меню бота: балансир-группа схлопнута в одно
// имя, порядок как в опросе, без дублей.
func (tb *Bot) groupNames() []string {
	cur, _ := tb.st.CurrentRows()
	seen := map[string]bool{}
	var out []string
	for _, r := range cur {
		name := groupDisplay(r)
		if seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	return out
}

// resolveName находит имя сервера (или балансир-группы) по его токену среди
// текущих серверов. Пустая строка — если сервер исчез между отрисовкой меню и
// нажатием (тогда вызывающий просто перерисует меню без изменений).
func (tb *Bot) resolveName(tok string) string {
	for _, name := range tb.groupNames() {
		if nameTok(name) == tok {
			return name
		}
	}
	return ""
}
