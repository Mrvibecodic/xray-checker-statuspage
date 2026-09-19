package bot

import (
	"strings"
	"testing"

	"xray-status/internal/summary"
)

// Переименование из бота: кнопка → ожидание ввода → алиас по имени группы
// (для балансира — одна запись на группу) → видно в summary; сброс кнопкой.
func TestRename_flow(t *testing.T) {
	tb := newGroupBot(t)

	txt, kb := tb.handleRenCallback(1, 77, "ren:"+nameTok("Bal"))
	if !strings.Contains(txt, "Bal") || strings.Contains(flatKB(kb), "Исходное") {
		t.Fatalf("подсказка: %q / %q", txt, flatKB(kb))
	}
	if tb.st.GetBotState(1, "await") != "ren_name" || tb.st.GetBotState(1, "await_msg") != "77" {
		t.Fatal("режим ожидания не включён")
	}
	tb.applyRename(1, "  Балансир NL  ")
	if tb.st.GetBotState(1, "ren_target") != "" {
		t.Fatal("цель не очищена")
	}
	al, _ := tb.st.Aliases()
	if al["Bal"] != "Балансир NL" || len(al) != 1 {
		t.Fatalf("алиас: %v", al)
	}
	if !strings.Contains(flatKB(tb.renKB(1)), "Bal → Балансир NL") {
		t.Fatalf("список: %s", flatKB(tb.renKB(1)))
	}

	p, err := summary.BuildSummary(tb.st, tb.cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, s := range p["servers"].([]any) {
		names = append(names, s.(map[string]any)["name"].(string))
	}
	if strings.Join(names, ",") != "Балансир NL,Solo" {
		t.Fatalf("страница: %v", names)
	}

	_, kb = tb.handleRenCallback(1, 77, "ren:"+nameTok("Bal"))
	if !strings.Contains(flatKB(kb), "Исходное") {
		t.Fatal("нет кнопки сброса")
	}
	tb.handleRenCallback(1, 77, "ren:reset:"+nameTok("Bal"))
	if al, _ := tb.st.Aliases(); len(al) != 0 {
		t.Fatalf("сброс: %v", al)
	}

	// исчезнувший сервер / пустой ввод — без записи
	tb.handleRenCallback(1, 77, "ren:deadbeef")
	tb.applyRename(1, "X")
	if al, _ := tb.st.Aliases(); len(al) != 0 {
		t.Fatalf("запись без цели: %v", al)
	}
}
