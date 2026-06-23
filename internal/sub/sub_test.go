package sub

import (
	"encoding/base64"
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

func TestNames(t *testing.T) {
	names := Names([]byte("vless://a#NL\nvless://b#DE\nvless://c#NL"))
	if len(names) != 2 || names[0] != "NL" || names[1] != "DE" {
		t.Fatalf("names: %v", names)
	}
}
