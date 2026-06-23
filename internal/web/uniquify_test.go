package web

import (
	"strings"
	"testing"
)

func TestUniquify(t *testing.T) {
	in := `.lock{} .lockon{} class="overall" locksmith blocklock`
	out := uniquify(in, []string{"lock", "lockon", "overall"})

	// Подстроки внутри слов не трогаем (границы [^\w-]).
	if !strings.Contains(out, "locksmith") || !strings.Contains(out, "blocklock") {
		t.Errorf("подстрочный 'lock' ошибочно заменён: %q", out)
	}
	// Самостоятельные токены заменены целиком (исходный текст исчез).
	if strings.Contains(out, "overall") {
		t.Errorf("токен 'overall' не заменён: %q", out)
	}
	if strings.Contains(out, ".lock{") || strings.Contains(out, ".lockon{") {
		t.Errorf("самостоятельный lock/lockon не заменён: %q", out)
	}
}

func TestUniquifyConsistentAndIndependent(t *testing.T) {
	in := `a overall b overall c row`
	out := uniquify(in, []string{"overall", "row"})
	f := strings.Fields(out)
	// один и тот же токен -> одно и то же имя в обоих местах
	if f[1] != f[3] {
		t.Errorf("'overall' заменён несогласованно: %q", out)
	}
	// разные токены -> разные имена (нет общего префикса)
	if f[1] == f[5] {
		t.Errorf("разные токены получили одинаковое имя: %q", out)
	}
}
