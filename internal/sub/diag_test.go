package sub

import (
	"encoding/base64"
	"testing"
)

func TestDiagnose_jsonWithBalancer(t *testing.T) {
	raw := []byte(`[
	  {"remarks":"Group A","outbounds":[{"tag":"proxy","protocol":"vless"},{"tag":"proxy-2","protocol":"vless"},{"tag":"proxy-3","protocol":"trojan"},{"tag":"direct","protocol":"freedom"},{"tag":"block","protocol":"blackhole"}]},
	  {"remarks":"Solo","outbounds":[{"tag":"proxy","protocol":"vless"},{"tag":"direct","protocol":"freedom"}]}
	]`)
	d := Diagnose(raw)
	if !d.JSON {
		t.Fatal("JSON-подписка не распознана")
	}
	if len(d.Configs) != 2 {
		t.Fatalf("ожидалось 2 конфига, got %d", len(d.Configs))
	}
	if d.Configs[0].Remarks != "Group A" || d.Configs[0].Nodes != 3 {
		t.Fatalf("балансер: %+v", d.Configs[0])
	}
	if d.Configs[1].Remarks != "Solo" || d.Configs[1].Nodes != 1 {
		t.Fatalf("одиночный: %+v", d.Configs[1])
	}
}

func TestDiagnose_base64Links(t *testing.T) {
	raw := []byte(base64.StdEncoding.EncodeToString([]byte("vless://x@a:1#A\nvless://y@b:2#B\n")))
	d := Diagnose(raw)
	if d.JSON {
		t.Fatal("base64-подписка не должна считаться JSON")
	}
	if d.Links != 2 {
		t.Fatalf("ожидалось 2 ссылки, got %d", d.Links)
	}
}

func TestDiagnose_envelope(t *testing.T) {
	raw := []byte(`{"data":[{"remarks":"A","outbounds":[{"tag":"proxy","protocol":"vless"}]}]}`)
	d := Diagnose(raw)
	if !d.JSON || len(d.Configs) != 1 || d.Configs[0].Nodes != 1 {
		t.Fatalf("конверт data: %+v", d)
	}
}
