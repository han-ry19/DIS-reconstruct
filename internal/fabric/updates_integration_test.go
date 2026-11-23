package fabric

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/han-ry19/DIS-reconstruct/internal/domain"
	mdns "github.com/miekg/dns"
)

func TestLiveFabricManagementUpdates(t *testing.T) {
	path := os.Getenv("DIS_FABRIC_CONFIG")
	if path == "" {
		t.Skip("set DIS_FABRIC_CONFIG to a network running the management-update chaincode")
	}
	backend, e := Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer backend.Close()
	initial, e := backend.View()
	if e != nil {
		t.Fatal(e)
	}
	var random [8]byte
	if _, e = rand.Read(random[:]); e != nil {
		t.Fatal(e)
	}
	suffix := hex.EncodeToString(random[:])
	ownerID := "owner-" + suffix + ".uid"
	nextID := "next-" + suffix + ".uid"
	appID := "app-" + suffix + ".uid"
	otherID := "other-" + suffix + ".uid"
	dataID := "item-" + suffix + ".data"
	keys := map[string]ed25519.PrivateKey{}
	calls := 0
	submit := func(op, actor string, p domain.Parameters) (domain.Request, domain.Result) {
		t.Helper()
		calls++
		r := domain.Request{Domain: initial.Domain, Operation: op, Actor: actor, RequestID: fmt.Sprintf("management-%s-%d", suffix, calls), Timestamp: time.Now().Unix(), Parameters: p}
		if e = domain.Sign(&r, keys[actor]); e != nil {
			t.Fatal(e)
		}
		result, e := backend.Submit(r)
		if e != nil {
			t.Fatalf("%s/%s: %v", op, p.Field, e)
		}
		return r, result
	}
	for i, id := range []string{ownerID, nextID, appID, otherID} {
		pub, k, e := ed25519.GenerateKey(rand.Reader)
		if e != nil {
			t.Fatal(e)
		}
		keys[id] = k
		kind := "user"
		if i >= 2 {
			kind = "application"
		}
		submit("identity.register", id, domain.Parameters{IdentityID: id, Kind: kind, PublicKey: base64.StdEncoding.EncodeToString(pub)})
	}
	sig := base64.StdEncoding.EncodeToString(ed25519.Sign(keys[ownerID], []byte("original")))
	submit("data.register", ownerID, domain.Parameters{DataID: dataID, Address: "https://example.org/old", DataSignature: sig, Profile: &domain.DataProfile{Label: "Management"}})
	submit("authorization.request", appID, domain.Parameters{DataID: dataID, Recipient: appID, Permissions: []string{"Read"}, ExpiresAt: time.Now().Unix() + 3600})
	submit("authorization.decide", ownerID, domain.Parameters{DataID: dataID, Recipient: appID, Status: domain.AuthorizationApproved})
	address := os.Getenv("DIS_TEST_DNS_ADDRESS")
	waitDNS := func(name string, kind uint16, predicate func([]mdns.RR) bool) {
		t.Helper()
		if address == "" {
			return
		}
		deadline := time.Now().Add(30 * time.Second)
		client := mdns.Client{Timeout: time.Second}
		for {
			msg := new(mdns.Msg)
			msg.SetQuestion(mdns.Fqdn(name), kind)
			answer, _, err := client.Exchange(msg, address)
			if err == nil && answer.Authoritative && (answer.Rcode == mdns.RcodeSuccess || answer.Rcode == mdns.RcodeNameError) && predicate(answer.Answer) {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("DNS did not publish %s %s: %v %v", name, mdns.TypeToString[kind], answer, err)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	authName := func(recipient string) string { return strings.TrimSuffix(recipient, ".uid") + "." + dataID }
	authMatches := func(recipient string, status domain.AuthorizationStatus, permissions int) func([]mdns.RR) bool {
		return func(rrs []mdns.RR) bool {
			if len(rrs) != 1 {
				return false
			}
			txt, ok := rrs[0].(*mdns.TXT)
			if !ok {
				return false
			}
			// miekg/dns exposes received TXT strings in escaped presentation form.
			raw, err := strconv.Unquote("\"" + strings.Join(txt.Txt, "") + "\"")
			if err != nil {
				return false
			}
			var a domain.AuthorizationRecord
			if json.Unmarshal([]byte(raw), &a) != nil {
				return false
			}
			return a.Recipient == recipient && a.Status == status && len(a.Permissions) == permissions
		}
	}
	waitDNS(authName(appID), mdns.TypeTXT, authMatches(appID, domain.AuthorizationApproved, 1))
	submit("data.update", ownerID, domain.Parameters{DataID: dataID, Field: "URI", NewContent: "https://example.org/new"})
	waitDNS(dataID, mdns.TypeURI, func(rrs []mdns.RR) bool {
		if len(rrs) != 1 {
			return false
		}
		v, ok := rrs[0].(*mdns.URI)
		return ok && v.Target == "https://example.org/new"
	})
	sig = base64.StdEncoding.EncodeToString(ed25519.Sign(keys[ownerID], []byte("updated")))
	submit("data.update", ownerID, domain.Parameters{DataID: dataID, Field: "TXT", NewContent: sig})
	waitDNS(dataID, mdns.TypeTXT, func(rrs []mdns.RR) bool {
		if len(rrs) != 1 {
			return false
		}
		v, ok := rrs[0].(*mdns.TXT)
		return ok && strings.Join(v.Txt, "") == sig
	})
	submit("authorization.update", ownerID, domain.Parameters{DataID: dataID, Recipient: appID, Field: "permissions", NewContent: "[\"Read\",\"Update\"]"})
	submit("authorization.update", ownerID, domain.Parameters{DataID: dataID, Recipient: appID, Field: "recipient", NewContent: otherID})
	waitDNS(authName(appID), mdns.TypeTXT, func(rrs []mdns.RR) bool { return len(rrs) == 0 })
	waitDNS(authName(otherID), mdns.TypeTXT, authMatches(otherID, domain.AuthorizationApproved, 2))
	if _, e = backend.GetAuthorization(dataID, ownerID, appID); !errors.Is(e, domain.ErrNotFound) {
		t.Fatal("old recipient remains in ledger", e)
	}
	submit("data.update", ownerID, domain.Parameters{DataID: dataID, Field: "RP", NewContent: nextID})
	waitDNS(dataID, mdns.TypeRP, func(rrs []mdns.RR) bool {
		if len(rrs) != 1 {
			return false
		}
		v, ok := rrs[0].(*mdns.RP)
		return ok && v.Mbox == mdns.Fqdn(nextID)
	})
	p := domain.Parameters{DataID: dataID, AuthorizationOwner: ownerID, Recipient: otherID, Field: "status", NewContent: "rejected"}
	denied := domain.Request{Domain: initial.Domain, Operation: "authorization.update", Actor: ownerID, RequestID: "denied-" + suffix, Timestamp: time.Now().Unix(), Parameters: p}
	domain.Sign(&denied, keys[ownerID])
	if _, e = backend.Submit(denied); !errors.Is(e, domain.ErrUnauthorized) {
		t.Fatal("former owner retained management rights", e)
	}
	submit("authorization.update", nextID, p)
	waitDNS(authName(otherID), mdns.TypeTXT, authMatches(otherID, domain.AuthorizationRejected, 2))
	p.Field = "expires_at"
	p.NewContent = "1"
	last, result := submit("authorization.update", nextID, p)
	// Reopening the gateway preserves the exact result of a previously committed request.
	reopened, e := Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	retry, e := reopened.Submit(last)
	if e != nil || retry != result {
		t.Fatal("reconnect retry changed result", retry, e)
	}
	final, e := backend.View()
	if e != nil {
		t.Fatal(e)
	}
	d := final.Data[dataID]
	a := final.Authorizations[domain.AuthorizationKey(dataID, ownerID, otherID)]
	if d.OwnerID != nextID || d.Address != "https://example.org/new" || d.DataSig != sig || a.ExpiresAt != 1 || a.Status != domain.AuthorizationRejected {
		t.Fatal(d, a)
	}
	if final.Sequence != initial.Sequence+uint64(calls) {
		t.Fatal("rejected request or retry changed state", initial.Sequence, final.Sequence, calls)
	}
	t.Logf("verified %d real Fabric commits, all seven update fields, ownership checks, recipient removal, gateway reconnect, DNS=%s", calls, address)
}
