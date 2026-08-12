package bot

import (
	"strconv"
	"testing"

	"xray-status/internal/config"
	"xray-status/internal/store"
	"xray-status/internal/storetest"
)

// TestIncSetStatusValidation — callback inc:setst принимает только канонический
// статус; мусорный статус не пишет запись в ленту, валидный — пишет и меняет
// статус инцидента.
func TestIncSetStatusValidation(t *testing.T) {
	st, err := store.Open(storetest.DSN(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	tb := &Bot{st: st, cfg: config.Config{TZ: "UTC"}, admins: map[int64]bool{1: true}}

	id, err := st.CreateIncident("Test", "minor", nil, "создан", 1, false)
	if err != nil {
		t.Fatal(err)
	}
	sid := strconv.FormatInt(id, 10)

	ups, _ := st.IncidentUpdates(id)
	base := len(ups) // запись о создании

	// Мусорный статус — лента не должна вырасти.
	tb.handleIncCallback(1, 10, "inc:setst:"+sid+":bogus")
	if ups, _ = st.IncidentUpdates(id); len(ups) != base {
		t.Fatalf("невалидный статус не должен писать в ленту: было %d, стало %d", base, len(ups))
	}

	// Валидный статус — запись добавлена, статус инцидента обновлён.
	tb.handleIncCallback(1, 10, "inc:setst:"+sid+":monitoring")
	if ups, _ = st.IncidentUpdates(id); len(ups) != base+1 {
		t.Fatalf("валидный статус должен добавить запись: было %d, стало %d", base, len(ups))
	}
	if in, _ := st.GetIncident(id); in.Status != "monitoring" {
		t.Fatalf("статус должен стать monitoring, стал %q", in.Status)
	}
}
