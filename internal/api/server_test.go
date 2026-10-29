package api_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"github.com/han-ry19/DIS-reconstruct/internal/api"
	"github.com/han-ry19/DIS-reconstruct/internal/domain"
	"github.com/han-ry19/DIS-reconstruct/internal/store"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestRegistrationAuthorizationAndRecovery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	st, e := store.Open(path, "test-dis")
	if e != nil {
		t.Fatal(e)
	}
	if other, e := store.Open(path, "test-dis"); e == nil {
		other.Close()
		t.Fatal("second writer acquired lock")
	}
	server := httptest.NewServer(api.New(st))
	defer func() { server.Close(); st.Close() }()
	ownerPub, owner, _ := ed25519.GenerateKey(rand.Reader)
	appPub, app, _ := ed25519.GenerateKey(rand.Reader)
	_, outsider, _ := ed25519.GenerateKey(rand.Reader)
	now := time.Now().Unix()
	makeReq := func(op, actor, id string, p domain.Parameters, key ed25519.PrivateKey) domain.Request {
		r := domain.Request{Domain: "test-dis", Operation: op, Actor: actor, RequestID: id, Timestamp: now, Parameters: p}
		if e := domain.Sign(&r, key); e != nil {
			t.Fatal(e)
		}
		return r
	}
	submit := func(r domain.Request, code int) domain.Result {
		t.Helper()
		b, _ := json.Marshal(r)
		resp, e := http.Post(server.URL+"/v1/transactions", "application/json", bytes.NewReader(b))
		if e != nil {
			t.Fatal(e)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != code {
			t.Fatalf("%s: got %d want %d: %s", r.Operation, resp.StatusCode, code, body)
		}
		var result domain.Result
		if code == 200 {
			if e = json.Unmarshal(body, &result); e != nil {
				t.Fatal(e)
			}
		}
		return result
	}
	ownerRegistration := makeReq("identity.register", "alice.uid", "owner", domain.Parameters{IdentityID: "alice.uid", PublicKey: base64.StdEncoding.EncodeToString(ownerPub), Kind: "user"}, owner)
	submit(ownerRegistration, 200)
	submit(makeReq("identity.register", "app.uid", "app", domain.Parameters{IdentityID: "app.uid", PublicKey: base64.StdEncoding.EncodeToString(appPub), Kind: "application"}, app), 200)
	dataSig := base64.StdEncoding.EncodeToString(ed25519.Sign(owner, []byte("data payload")))
	registration := makeReq("data.register", "alice.uid", "data", domain.Parameters{DataID: "sample.data", Address: "https://data.example/item", DataSignature: dataSig, Profile: &domain.DataProfile{Label: "sample", Format: "text", Description: "Example data"}}, owner)
	result := submit(registration, 200)
	if replay := submit(registration, 200); replay != result {
		t.Fatal("replay changed result")
	}
	altered := registration
	altered.Parameters.Address = "https://data.example/changed"
	domain.Sign(&altered, owner)
	submit(altered, 409)
	duplicate := registration
	duplicate.RequestID = "duplicate"
	domain.Sign(&duplicate, owner)
	submit(duplicate, 409)
	tampered := registration
	tampered.RequestID = "tampered"
	submit(tampered, 403)
	wrongDomain := registration
	wrongDomain.Domain = "different"
	domain.Sign(&wrongDomain, owner)
	submit(wrongDomain, 400)
	expired := registration
	expired.RequestID = "stale"
	expired.Timestamp = now - 1000
	domain.Sign(&expired, owner)
	submit(expired, 400)
	badPermissions := makeReq("authorization.request", "app.uid", "bad-permissions", domain.Parameters{DataID: "sample.data", Recipient: "app.uid", Permissions: []string{"Root"}, ExpiresAt: now + 3600}, app)
	submit(badPermissions, 400)
	request := makeReq("authorization.request", "app.uid", "request", domain.Parameters{DataID: "sample.data", Recipient: "app.uid", Permissions: []string{"Read"}, ExpiresAt: now + 3600}, app)
	submit(request, 200)
	fetchAuth := func() domain.AuthorizationRecord {
		t.Helper()
		resp, e := http.Get(server.URL + "/v1/authorizations?data_id=sample.data&owner_id=alice.uid&recipient=app.uid")
		if e != nil {
			t.Fatal(e)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatal(resp.Status)
		}
		var a domain.AuthorizationRecord
		if e = json.NewDecoder(resp.Body).Decode(&a); e != nil {
			t.Fatal(e)
		}
		return a
	}
	pending := fetchAuth()
	if pending.Status != domain.AuthorizationPending || pending.IssuedAt != 0 {
		t.Fatal(pending)
	}
	decision := makeReq("authorization.decide", "alice.uid", "decision", domain.Parameters{DataID: "sample.data", Recipient: "app.uid", Status: domain.AuthorizationApproved}, owner)
	unauthorized := decision
	unauthorized.RequestID = "forged"
	domain.Sign(&unauthorized, outsider)
	submit(unauthorized, 403)
	appDecision := makeReq("authorization.decide", "app.uid", "app-decision", decision.Parameters, app)
	submit(appDecision, 403)
	accepted := submit(decision, 200)
	approved := fetchAuth()
	if approved.Status != domain.AuthorizationApproved || approved.ExpiresAt != pending.ExpiresAt || !reflect.DeepEqual(approved.Permissions, pending.Permissions) || approved.IssuedAt == 0 {
		t.Fatal(approved)
	}
	server.Close()
	if e = st.Close(); e != nil {
		t.Fatal(e)
	}
	st, e = store.Open(path, "test-dis")
	if e != nil {
		t.Fatal(e)
	}
	server = httptest.NewServer(api.New(st))
	if replay := submit(decision, 200); replay != accepted {
		t.Fatal("recovery changed receipt")
	}
	if got := fetchAuth(); !reflect.DeepEqual(got, approved) {
		t.Fatal("recovery changed authorization")
	}
	rejected := decision
	rejected.RequestID = "reject"
	rejected.Parameters.Status = domain.AuthorizationRejected
	domain.Sign(&rejected, owner)
	submit(rejected, 200)
	if fetchAuth().Status != domain.AuthorizationRejected {
		t.Fatal("rejection not stored")
	}
	// A newly signed application request writes pending at the same composite key,
	// as specified by DIA Algorithm 2; an exact transport retry retains its receipt.
	renewal := request
	renewal.RequestID = "renewal"
	domain.Sign(&renewal, app)
	submit(renewal, 200)
	if fetchAuth().Status != domain.AuthorizationPending {
		t.Fatal("renewal not pending")
	}
	submit(renewal, 200)
	state, e := st.View()
	if e != nil {
		t.Fatal(e)
	}
	if state.Sequence != 7 {
		t.Fatalf("unexpected sequence: %d", state.Sequence)
	}
}

func TestConcurrentRegistrationAndRollback(t *testing.T) {
	st, e := store.Open(filepath.Join(t.TempDir(), "state.json"), "test")
	if e != nil {
		t.Fatal(e)
	}
	defer st.Close()
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	timestamp := time.Now().Unix()
	var wg sync.WaitGroup
	results := make(chan error, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := domain.Request{Domain: "test", Operation: "identity.register", Actor: "alice.uid", RequestID: "same", Timestamp: timestamp, Parameters: domain.Parameters{IdentityID: "alice.uid", PublicKey: base64.StdEncoding.EncodeToString(pub), Kind: "user"}}
			domain.Sign(&r, key)
			_, err := st.Update(func(s *domain.State) (domain.Result, error) { return domain.Apply(s, r, r.Timestamp) })
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	for e := range results {
		if e != nil {
			t.Fatal(e)
		}
	}
	s, _ := st.View()
	if s.Sequence != 1 {
		t.Fatal("duplicate concurrent commits")
	}
	_, e = st.Update(func(s *domain.State) (domain.Result, error) {
		s.Sequence = 99
		return domain.Result{}, domain.ErrInvalid
	})
	if e == nil {
		t.Fatal("expected callback error")
	}
	s, _ = st.View()
	if s.Sequence != 1 {
		t.Fatal("failed update changed state")
	}
	s.Identities["mutated"] = domain.Identity{}
	again, _ := st.View()
	if _, ok := again.Identities["mutated"]; ok {
		t.Fatal("query leaked mutable state")
	}
}
