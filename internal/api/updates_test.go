package api_test

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/han-ry19/DIS-reconstruct/internal/api"
	"github.com/han-ry19/DIS-reconstruct/internal/domain"
	"github.com/han-ry19/DIS-reconstruct/internal/store"
)

func TestManagementUpdatesThroughHTTPAndRecovery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "management.json")
	st, e := store.Open(path, "http-updates")
	if e != nil {
		t.Fatal(e)
	}
	server := httptest.NewServer(api.New(st))
	defer func() { server.Close(); st.Close() }()
	keys := map[string]ed25519.PrivateKey{}
	n := 0
	submit := func(op, actor string, p domain.Parameters, status int) domain.Request {
		t.Helper()
		n++
		r := domain.Request{Domain: "http-updates", Operation: op, Actor: actor, RequestID: fmt.Sprintf("http-%d", n), Timestamp: time.Now().Unix(), Parameters: p}
		if e := domain.Sign(&r, keys[actor]); e != nil {
			t.Fatal(e)
		}
		b, _ := json.Marshal(r)
		res, e := http.Post(server.URL+"/v1/transactions", "application/json", bytes.NewReader(b))
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		if res.StatusCode != status {
			t.Fatalf("%s %s: got %d want %d", op, p.Field, res.StatusCode, status)
		}
		return r
	}
	get := func(route string, status int, target any) {
		t.Helper()
		r, e := http.Get(server.URL + route)
		if e != nil {
			t.Fatal(e)
		}
		defer r.Body.Close()
		if r.StatusCode != status {
			t.Fatalf("GET %s returned %d", route, r.StatusCode)
		}
		if target != nil {
			if e = json.NewDecoder(r.Body).Decode(target); e != nil {
				t.Fatal(e)
			}
		}
	}
	for i, id := range []string{"alice.uid", "bob.uid", "app.uid", "other.uid"} {
		seed := make([]byte, 32)
		seed[0] = byte(i + 1)
		k := ed25519.NewKeyFromSeed(seed)
		keys[id] = k
		kind := "user"
		if i >= 2 {
			kind = "application"
		}
		submit("identity.register", id, domain.Parameters{IdentityID: id, Kind: kind, PublicKey: base64.StdEncoding.EncodeToString(k.Public().(ed25519.PublicKey))}, 200)
	}
	submit("data.register", "alice.uid", domain.Parameters{DataID: "item.data", Address: "https://example.org/old", DataSignature: base64.StdEncoding.EncodeToString(make([]byte, 64)), Profile: &domain.DataProfile{}}, 200)
	submit("authorization.request", "app.uid", domain.Parameters{DataID: "item.data", Recipient: "app.uid", Permissions: []string{"Read"}, ExpiresAt: time.Now().Unix() + 3600}, 200)
	submit("authorization.decide", "alice.uid", domain.Parameters{DataID: "item.data", Recipient: "app.uid", Status: domain.AuthorizationApproved}, 200)
	submit("data.update", "app.uid", domain.Parameters{DataID: "item.data", Field: "URI", NewContent: "https://example.org/new"}, 403)
	submit("data.update", "alice.uid", domain.Parameters{DataID: "item.data", Field: "URI", NewContent: "relative"}, 400)
	submit("data.update", "alice.uid", domain.Parameters{DataID: "missing.data", Field: "URI", NewContent: "https://example.org/new"}, 404)
	submit("data.update", "alice.uid", domain.Parameters{DataID: "item.data", Field: "URI", NewContent: "https://example.org/new"}, 200)
	submit("authorization.update", "alice.uid", domain.Parameters{DataID: "item.data", Recipient: "app.uid", Field: "permissions", NewContent: "[\"Read\",\"Update\"]"}, 200)
	submit("authorization.update", "alice.uid", domain.Parameters{DataID: "item.data", Recipient: "app.uid", Field: "recipient", NewContent: "other.uid"}, 200)
	submit("data.update", "alice.uid", domain.Parameters{DataID: "item.data", Field: "RP", NewContent: "bob.uid"}, 200)
	submit("authorization.update", "alice.uid", domain.Parameters{DataID: "item.data", Recipient: "other.uid", AuthorizationOwner: "alice.uid", Field: "status", NewContent: "rejected"}, 403)
	last := submit("authorization.update", "bob.uid", domain.Parameters{DataID: "item.data", Recipient: "other.uid", AuthorizationOwner: "alice.uid", Field: "status", NewContent: "rejected"}, 200)
	oldRoute := "/v1/authorizations?data_id=item.data&owner_id=alice.uid&recipient=app.uid"
	route := "/v1/authorizations?data_id=item.data&owner_id=alice.uid&recipient=other.uid"
	get(oldRoute, 404, nil)
	var a domain.AuthorizationRecord
	get(route, 200, &a)
	if a.Status != domain.AuthorizationRejected || a.Recipient != "other.uid" || len(a.Permissions) != 2 {
		t.Fatal(a)
	}
	var d domain.DataIdentifier
	get("/v1/data/item.data", 200, &d)
	if d.OwnerID != "bob.uid" || d.Address != "https://example.org/new" {
		t.Fatal(d)
	}
	before, e := st.View()
	if e != nil {
		t.Fatal(e)
	}
	server.Close()
	if e = st.Close(); e != nil {
		t.Fatal(e)
	}
	st, e = store.Open(path, "http-updates")
	if e != nil {
		t.Fatal(e)
	}
	server = httptest.NewServer(api.New(st))
	get(oldRoute, 404, nil)
	get(route, 200, &a)
	if a.Status != domain.AuthorizationRejected {
		t.Fatal("recovery lost revocation")
	}
	b, _ := json.Marshal(last)
	res, e := http.Post(server.URL+"/v1/transactions", "application/json", bytes.NewReader(b))
	if e != nil {
		t.Fatal(e)
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatal("retry after recovery failed", res.Status)
	}
	after, e := st.View()
	if e != nil {
		t.Fatal(e)
	}
	x, _ := json.Marshal(before)
	y, _ := json.Marshal(after)
	if string(x) != string(y) {
		t.Fatal("recovery retry changed persisted state")
	}
}
