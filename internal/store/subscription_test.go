package store

import (
	"testing"
	"xray-status/internal/storetest"
)

func TestSubscriptionURLSecret(t *testing.T) {
	st, err := Open(storetest.DSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	_ = st.EnableSecrets("00000000000000000000000000000000000000000000000000000000000000bb")

	if st.HasSubscriptionURL() {
		t.Fatal("no sub url yet")
	}
	if err := st.SetSubscriptionURL("https://provider/sub?token=abc"); err != nil {
		t.Fatal(err)
	}
	if !st.HasSubscriptionURL() {
		t.Fatal("sub url should be set")
	}
	got, _ := st.SubscriptionURL()
	if got != "https://provider/sub?token=abc" {
		t.Fatalf("sub url: %q", got)
	}
}

func TestMultipleSubscriptions(t *testing.T) {
	st, err := Open(storetest.DSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	_ = st.EnableSecrets("00000000000000000000000000000000000000000000000000000000000000bb")

	// существующий одиночный секрет читается как список из одного элемента
	_ = st.SetSubscriptionURL("https://a/sub?t=1")
	if got := st.SubscriptionURLs(); len(got) != 1 || got[0] != "https://a/sub?t=1" {
		t.Fatalf("single->list: %v", got)
	}

	// добавление не перезатирает и не дублирует
	n, err := st.AddSubscriptionURLs([]string{"https://b/sub", "https://a/sub?t=1", "https://c/sub"})
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("added = %d, want 2", n)
	}
	got := st.SubscriptionURLs()
	if len(got) != 3 || got[0] != "https://a/sub?t=1" || got[1] != "https://b/sub" || got[2] != "https://c/sub" {
		t.Fatalf("after add: %v", got)
	}

	// удаление по индексу
	removed, err := st.RemoveSubscriptionURLAt(1)
	if err != nil || removed != "https://b/sub" {
		t.Fatalf("removed = %q, err %v", removed, err)
	}
	got = st.SubscriptionURLs()
	if len(got) != 2 || got[0] != "https://a/sub?t=1" || got[1] != "https://c/sub" {
		t.Fatalf("after remove: %v", got)
	}

	// удаление вне диапазона — no-op
	if r, _ := st.RemoveSubscriptionURLAt(9); r != "" {
		t.Fatalf("out-of-range remove returned %q", r)
	}

	// удалить всё → подписок нет
	_, _ = st.RemoveSubscriptionURLAt(0)
	_, _ = st.RemoveSubscriptionURLAt(0)
	if st.HasSubscriptionURL() || len(st.SubscriptionURLs()) != 0 {
		t.Fatalf("should be empty: has=%v list=%v", st.HasSubscriptionURL(), st.SubscriptionURLs())
	}
}
