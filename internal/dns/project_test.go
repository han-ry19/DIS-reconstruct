package dns

import (
	"encoding/base64"
	"encoding/json"
	"github.com/han-ry19/DIS-reconstruct/internal/domain"
	mdns "github.com/miekg/dns"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func fixture() domain.State {
	s := domain.NewState("test")
	s.Sequence = 5
	s.Identities["alice.uid"] = domain.Identity{ID: "alice.uid", PublicKey: base64.StdEncoding.EncodeToString(make([]byte, 32)), Kind: "user"}
	s.Identities["app.uid"] = domain.Identity{ID: "app.uid", PublicKey: base64.StdEncoding.EncodeToString(make([]byte, 32)), Kind: "application"}
	s.Data["sample.data"] = domain.DataIdentifier{ID: "sample.data", OwnerID: "alice.uid", Address: "https://example.org/data", DataSig: strings.Repeat("x", 400), Profile: domain.DataProfile{Description: strings.Repeat("quote \" \\ \n", 60)}}
	a := domain.AuthorizationRecord{DataID: "sample.data", OwnerID: "alice.uid", Recipient: "app.uid", Status: domain.AuthorizationApproved, Permissions: []string{"Read"}, IssuedAt: 1, ExpiresAt: 100}
	s.Authorizations[domain.AuthorizationKey(a.DataID, a.OwnerID, a.Recipient)] = a
	return s
}
func TestProjectionAndPublication(t *testing.T) {
	s := fixture()
	zones, e := Project(s)
	if e != nil {
		t.Fatal(e)
	}
	if len(zones["uid."]) != 7 || len(zones["data."]) != 7 {
		t.Fatal("missing records")
	}
	for _, records := range zones {
		for _, rr := range records {
			parsed, e := mdns.NewRR(rr.String())
			if e != nil {
				t.Fatal(e)
			}
			if parsed.String() != rr.String() {
				t.Fatal("zone encoding changed record")
			}
		}
	}
	a := s.Authorizations["sample.data|alice.uid|app.uid"]
	if AuthorizationName(a) != "app.sample.data." {
		t.Fatal("authorization naming")
	}
	dir := t.TempDir()
	if e = Publish(s, dir); e != nil {
		t.Fatal(e)
	}
	before, e := os.ReadFile(filepath.Join(dir, "data.zone"))
	if e != nil {
		t.Fatal(e)
	}
	if e = Publish(s, dir); e != nil {
		t.Fatal("repeat publication", e)
	}
	s.Sequence--
	if e = Publish(s, dir); e == nil {
		t.Fatal("stale state published")
	}
	s.Sequence++
	s.Data["extra.data"] = domain.DataIdentifier{ID: "extra.data", OwnerID: "alice.uid", Address: "https://example.org/extra"}
	if e = Publish(s, dir); e == nil {
		t.Fatal("different state at same sequence published")
	}
	after, _ := os.ReadFile(filepath.Join(dir, "data.zone"))
	if string(after) != string(before) {
		t.Fatal("failed publication changed zone")
	}
	s.Sequence++
	if e = Publish(s, dir); e != nil {
		t.Fatal(e)
	}
}
func TestInvalidNamesCannotEscapeZone(t *testing.T) {
	s := fixture()
	s.Identities["bad"] = domain.Identity{ID: "bad.example", PublicKey: base64.StdEncoding.EncodeToString(make([]byte, 32))}
	if _, e := Project(s); e == nil {
		t.Fatal("out-of-zone name accepted")
	}
}

// Verify the externally published TXT payload after DNS wire encoding.
func TestAuthorizationExpirationWireContract(t *testing.T) {
	s := fixture()
	zones, err := Project(s)
	if err != nil {
		t.Fatal(err)
	}
	var record *mdns.TXT
	for _, rr := range zones["data."] {
		if txt, ok := rr.(*mdns.TXT); ok && txt.Hdr.Name == "app.sample.data." {
			record = txt
		}
	}
	if record == nil {
		t.Fatal("missing authorization TXT")
	}
	msg := &mdns.Msg{Answer: []mdns.RR{record}}
	wire, err := msg.Pack()
	if err != nil {
		t.Fatal(err)
	}
	var decoded mdns.Msg
	if err := decoded.Unpack(wire); err != nil {
		t.Fatal(err)
	}
	raw, err := strconv.Unquote("\"" + strings.Join(decoded.Answer[0].(*mdns.TXT).Txt, "") + "\"")
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatal(err)
	}
	if string(payload["expired_at"]) != "100" {
		t.Fatalf("wrong paper expiration field: %s", raw)
	}
	if _, exists := payload["expires_at"]; exists {
		t.Fatal("internal expiration field leaked into DNS")
	}
	var consumer struct {
		ExpiredAt int64 `json:"expired_at"`
	}
	if err := json.Unmarshal([]byte(raw), &consumer); err != nil || consumer.ExpiredAt != 100 {
		t.Fatalf("consumer expiration: %+v, %v", consumer, err)
	}
	a := s.Authorizations["sample.data|alice.uid|app.uid"]
	stored, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(stored), "\"expires_at\":100") {
		t.Fatal("ledger encoding changed")
	}
}
