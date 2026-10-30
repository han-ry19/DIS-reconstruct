package disredis

import (
	"encoding/json"
	"github.com/miekg/dns"
	"testing"
)

func TestDecodeDISRecordTypes(t *testing.T) {
	raw := map[string]any{"cert": []any{map[string]any{"ttl": 3600, "type": 254, "keytag": 0, "algorithm": 0, "certificate": "YWJj"}}, "uri": []any{map[string]any{"ttl": 3600, "priority": 0, "weight": 0, "target": "https://example.org"}}, "rp": []any{map[string]any{"ttl": 3600, "mbox": "alice.uid.", "txt": "."}}, "txt": []any{map[string]any{"ttl": 3600, "text": "hello"}}}
	b, _ := json.Marshal(raw)
	rows, e := decode("sample.data.", b)
	if e != nil {
		t.Fatal(e)
	}
	if len(rows) != 4 {
		t.Fatal(rows)
	}
	for _, rr := range rows {
		if _, e = dns.NewRR(rr.String()); e != nil {
			t.Fatal(e)
		}
	}
}
