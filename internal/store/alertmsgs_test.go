package store

import (
	"path/filepath"
	"testing"
)

func TestAlertMessagesLifecycle(t *testing.T) {
	st, err := Open("sqlite", filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	if err := st.AddAlertMessage(100, 1, 1000); err != nil {
		t.Fatal(err)
	}
	if err := st.AddAlertMessage(100, 2, 2000); err != nil {
		t.Fatal(err)
	}

	due, err := st.DueAlertMessages(1500)
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 1 || due[0].MsgID != 1 || due[0].ChatID != 100 {
		t.Fatalf("due@1500: %+v", due)
	}

	if err := st.RemoveAlertMessage(100, 1); err != nil {
		t.Fatal(err)
	}
	due, _ = st.DueAlertMessages(3000)
	if len(due) != 1 || due[0].MsgID != 2 {
		t.Fatalf("после удаления должен остаться msg 2: %+v", due)
	}
}

func TestAlertTTLHoursClamp(t *testing.T) {
	st, err := Open("sqlite", filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	if got := st.AlertTTLHours(); got != 0 {
		t.Fatalf("дефолт должен быть 0 (выкл), got %d", got)
	}
	_ = st.SetAlertTTLHours(24)
	if got := st.AlertTTLHours(); got != 24 {
		t.Fatalf("ожидалось 24, got %d", got)
	}
	_ = st.SetAlertTTLHours(999)
	if got := st.AlertTTLHours(); got != 48 {
		t.Fatalf("потолок 48 (лимит Telegram), got %d", got)
	}
	_ = st.SetAlertTTLHours(-5)
	if got := st.AlertTTLHours(); got != 0 {
		t.Fatalf("отрицательное = 0, got %d", got)
	}
}
