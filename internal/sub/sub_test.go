package sub

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestFilterPlain(t *testing.T) {
	in := "vless://a@h:443#NL\nvless://b@h:443#DE"
	out := string(Filter([]byte(in), map[string]bool{"DE": true}))
	if out != "vless://a@h:443#NL" {
		t.Fatalf("plain filter: %q", out)
	}
}

func TestFilterBase64(t *testing.T) {
	plain := "vless://a@h:443#NL\nvless://b@h:443#DE"
	in := base64.StdEncoding.EncodeToString([]byte(plain))
	out := Filter([]byte(in), map[string]bool{"NL": true})
	dec, err := base64.StdEncoding.DecodeString(string(out))
	if err != nil {
		t.Fatalf("output not base64: %v", err)
	}
	if string(dec) != "vless://b@h:443#DE" {
		t.Fatalf("base64 filter: %q", dec)
	}
}

func TestRemarkURLDecode(t *testing.T) {
	if got := Remark("vless://a@h:443#NL%20Amsterdam"); got != "NL Amsterdam" {
		t.Fatalf("remark decode: %q", got)
	}
	out := string(Filter([]byte("vless://a#NL%20Amsterdam\nvless://b#DE"), map[string]bool{"NL Amsterdam": true}))
	if out != "vless://b#DE" {
		t.Fatalf("filter urlencoded remark: %q", out)
	}
}

func TestMerge(t *testing.T) {
	plain := "vless://a@h:443#NL\nvless://b@h:443#DE"
	b64 := base64.StdEncoding.EncodeToString([]byte("vless://c@h:443#FR\nvless://a@h:443#NL"))
	out := Merge([][]byte{[]byte(plain), []byte(b64)}, map[string]bool{"DE": true})
	dec, err := base64.StdEncoding.DecodeString(string(out))
	if err != nil {
		t.Fatalf("merge output not base64: %v", err)
	}
	// DE отфильтрован, дубль NL схлопнут, порядок сохранён по источникам.
	want := "vless://a@h:443#NL\nvless://c@h:443#FR"
	if string(dec) != want {
		t.Fatalf("merge = %q, want %q", dec, want)
	}
}

func TestMergeUniquifiesDuplicateRemarks(t *testing.T) {
	// Две разные строки с ОДИНАКОВОЙ ремаркой #NL из разных подписок.
	a := "vless://a@h1:443#NL"
	b := "vless://b@h2:443#NL"
	c := "vless://c@h3:443#DE" // уникальная — тег не добавляется
	out := Merge([][]byte{[]byte(a + "\n" + c), []byte(b)}, nil)
	dec, err := base64.StdEncoding.DecodeString(string(out))
	if err != nil {
		t.Fatalf("not base64: %v", err)
	}
	lines := splitLines(string(dec))
	foundDE := false
	tagged := map[string]bool{}
	for _, l := range lines {
		r := Remark(l)
		switch {
		case r == "DE":
			foundDE = true
		case r == "NL":
			t.Fatalf("duplicate remark NL must be tagged, got plain: %q", l)
		case len(r) > 3 && r[:3] == "NL ": // "NL ·#xxxx"
			tagged[l] = true
			if StripTag(r) != "NL" {
				t.Fatalf("StripTag(%q) = %q, want NL", r, StripTag(r))
			}
		}
	}
	if !foundDE {
		t.Fatal("DE line missing")
	}
	if len(tagged) != 2 {
		t.Fatalf("want 2 tagged NL lines, got %d (%v)", len(tagged), lines)
	}

	// Тег стабилен: повторный вызов даёт те же имена.
	out2, _ := base64.StdEncoding.DecodeString(string(Merge([][]byte{[]byte(a + "\n" + c), []byte(b)}, nil)))
	if string(out2) != string(dec) {
		t.Fatalf("merge not stable:\n%q\nvs\n%q", out2, dec)
	}

	// Выключение одного дубля по его тегированному имени убирает ТОЛЬКО его.
	var nlNames []string
	for l := range tagged {
		nlNames = append(nlNames, Remark(l))
	}
	dis := map[string]bool{nlNames[0]: true}
	out3, _ := base64.StdEncoding.DecodeString(string(Merge([][]byte{[]byte(a + "\n" + c), []byte(b)}, dis)))
	got := splitLines(string(out3))
	if len(got) != 2 { // один NL выключен, остаётся второй NL + DE
		t.Fatalf("disable one dup: want 2 lines, got %d (%v)", len(got), got)
	}
}

func TestStripTag(t *testing.T) {
	if StripTag("NL ·#a3f1") != "NL" {
		t.Fatalf("strip tagged: %q", StripTag("NL ·#a3f1"))
	}
	if StripTag("NL Amsterdam") != "NL Amsterdam" {
		t.Fatalf("must not strip plain name: %q", StripTag("NL Amsterdam"))
	}
}

func splitLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}

func TestParseURLs(t *testing.T) {
	cases := map[string][]string{
		"a, b ,c":         {"a", "b", "c"},
		"a\nb\r\nc":       {"a", "b", "c"},
		"  a   b  ":       {"a", "b"},
		"https://x?a=1,y": {"https://x?a=1", "y"},
		"":                nil,
	}
	for in, want := range cases {
		got := ParseURLs(in)
		if len(got) != len(want) {
			t.Fatalf("ParseURLs(%q) = %v, want %v", in, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("ParseURLs(%q)[%d] = %q, want %q", in, i, got[i], want[i])
			}
		}
	}
}

func TestNames(t *testing.T) {
	names := Names([]byte("vless://a#NL\nvless://b#DE\nvless://c#NL"))
	if len(names) != 2 || names[0] != "NL" || names[1] != "DE" {
		t.Fatalf("names: %v", names)
	}
}

func TestFilterJSON_keepsBalancerDropsDisabled(t *testing.T) {
	// XRAY_JSON-подписка: три конфига, у первого балансер. Отключаем второй.
	raw := []byte(`[
	  {"remarks":"Group A","routing":{"balancers":[{"tag":"bal","selector":["proxy"]}]},"outbounds":[{"tag":"proxy"},{"tag":"proxy-2"},{"tag":"direct"}]},
	  {"remarks":"Group B","outbounds":[{"tag":"proxy"}]},
	  {"remarks":"Group C","outbounds":[{"tag":"proxy"}]}
	]`)
	out, ok := FilterJSON([][]byte{raw}, map[string]bool{"Group B": true})
	if !ok {
		t.Fatal("FilterJSON должен распознать JSON-подписку")
	}
	var arr []map[string]any
	if err := json.Unmarshal(out, &arr); err != nil {
		t.Fatalf("выход не JSON: %v", err)
	}
	if len(arr) != 2 {
		t.Fatalf("ожидалось 2 конфига (отключённый убран), got %d", len(arr))
	}
	// балансер сохранён у первого
	r0 := arr[0]
	if r0["remarks"] != "Group A" {
		t.Fatalf("первый конфиг не балансер: %v", r0["remarks"])
	}
	if _, hasBal := r0["routing"].(map[string]any)["balancers"]; !hasBal {
		t.Fatal("routing.balancers потерян — балансер не дошёл бы до чекера")
	}
	// отключённого нет
	for _, c := range arr {
		if c["remarks"] == "Group B" {
			t.Fatal("отключённый сервер не должен попасть в выдачу")
		}
	}
}

func TestFilterJSON_base64NotJSON(t *testing.T) {
	// base64 share-подписка не должна распознаваться как JSON (идём по Merge-пути).
	b64 := []byte("dmxlc3M6Ly94QGE6MSNB") // base64 of one share-link with remark "A"
	if _, ok := FilterJSON([][]byte{b64}, nil); ok {
		t.Fatal("base64-подписка не должна обрабатываться как JSON")
	}
}
