package web

import (
	"strings"
	"testing"
)

// Правила из аудита анти-фингерпринта: эти идентификаторы НЕЛЬЗЯ вносить в
// uniqTokens — они совпадают с CSS-свойствами, JSON-ключами /api/summary или
// динамически собираемыми классами, и их рандомизация молча ломает рендер.
var forbiddenUniqTokens = map[string]bool{
	"top": true, "right": true, "wrap": true, "cursor": true,
	"name": true, "label": true, "stats": true, "title": true,
	"ok": true, "bad": true,
	"inc-minor": true, "inc-major": true, "inc-critical": true,
}

func TestUniqTokensHygiene(t *testing.T) {
	seen := map[string]bool{}
	for _, tok := range uniqTokens {
		if len(tok) < 2 {
			t.Errorf("токен %q слишком короткий — зацепит случайные совпадения", tok)
		}
		if forbiddenUniqTokens[tok] {
			t.Errorf("токен %q запрещён: совпадает с критичным идентификатором рендера", tok)
		}
		if seen[tok] {
			t.Errorf("токен %q продублирован", tok)
		}
		seen[tok] = true
	}
}

// containsWord — есть ли tok в s как отдельное «слово» (границы — не буквы,
// цифры, '_' и '-'; та же логика границ, что в uniquify).
func containsWord(s, tok string) bool {
	isWord := func(b byte) bool {
		return b == '_' || b == '-' ||
			(b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
	}
	for i := 0; i+len(tok) <= len(s); i++ {
		if s[i:i+len(tok)] != tok {
			continue
		}
		leftOK := i == 0 || !isWord(s[i-1])
		rightOK := i+len(tok) == len(s) || !isWord(s[i+len(tok)])
		if leftOK && rightOK {
			return true
		}
	}
	return false
}

// Для КАЖДОГО вшитого шаблона: после uniquify критичные JS/CSS-фрагменты
// остаются нетронутыми, а токены, встречавшиеся в шаблоне, — замангулены.
// Ловит две ошибки сразу: «вписали опасный токен» и «токен перестал матчиться».
func TestUniquifyTemplatesInvariants(t *testing.T) {
	files := []string{"assets/index.html.tpl", "assets/index2.html.tpl", "assets/index3.html.tpl"}
	musts := []string{"s.name", "d.label", "data.stats", "data.title", `style="top:`, "'inc-'+"}
	for _, f := range files {
		raw, err := assets.ReadFile(f)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		src := string(raw)
		out := uniquify(src, uniqTokens)
		for _, m := range musts {
			if strings.Contains(src, m) && !strings.Contains(out, m) {
				t.Errorf("%s: критичный фрагмент %q пропал после uniquify", f, m)
			}
		}
		for _, tok := range uniqTokens {
			if containsWord(src, tok) && containsWord(out, tok) {
				t.Errorf("%s: токен %q остался незамангуленным", f, tok)
			}
		}
	}
}
