package store

import (
	"strings"
	"testing"

	"xray-status/internal/storetest"
)

func TestAliases(t *testing.T) {
	st, err := Open(storetest.DSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if a, _ := st.Aliases(); len(a) != 0 {
		t.Fatalf("пусто на старте: %v", a)
	}
	if err := st.SetAlias("🇺🇸 США-2", "  США 2  "); err != nil {
		t.Fatal(err)
	}
	if err := st.SetAlias("🇺🇸 США-2", "Нью-Йорк 2"); err != nil { // перезапись
		t.Fatal(err)
	}
	if a, _ := st.Aliases(); a["🇺🇸 США-2"] != "Нью-Йорк 2" || len(a) != 1 {
		t.Fatalf("алиас: %v", a)
	}
	_ = st.SetAlias("Y", "Нью-Йорк\n\t 3")
	if a, _ := st.Aliases(); a["Y"] != "Нью-Йорк 3" {
		t.Fatalf("пробелы не схлопнуты: %q", a["Y"])
	}
	_ = st.SetAlias("Y", "")
	_ = st.SetAlias("X", strings.Repeat("я", AliasMaxRunes+10))
	if a, _ := st.Aliases(); len([]rune(a["X"])) != AliasMaxRunes {
		t.Fatalf("длина не обрезана: %d", len([]rune(a["X"])))
	}
	_ = st.SetAlias("🇺🇸 США-2", "")
	_ = st.SetAlias("X", "X") // совпадает с исходным = сброс
	if a, _ := st.Aliases(); len(a) != 0 {
		t.Fatalf("сброс не сработал: %v", a)
	}
}
