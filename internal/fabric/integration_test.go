package fabric

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"github.com/han-ry19/DIS-reconstruct/internal/domain"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestLiveFabricWorkflow(t *testing.T) {
	path := os.Getenv("DIS_FABRIC_CONFIG")
	if path == "" {
		t.Skip("set DIS_FABRIC_CONFIG to test a running network")
	}
	backend, e := Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer backend.Close()
	state, e := backend.View()
	if e != nil {
		t.Fatal(e)
	}
	var id [8]byte
	rand.Read(id[:])
	suffix := hex.EncodeToString(id[:])
	ownerID := "owner-" + suffix + ".uid"
	appID := "app-" + suffix + ".uid"
	dataID := "data-" + suffix + ".data"
	pub, owner, _ := ed25519.GenerateKey(rand.Reader)
	appPub, app, _ := ed25519.GenerateKey(rand.Reader)
	req := func(op, actor, call string, p domain.Parameters, key ed25519.PrivateKey) domain.Request {
		r := domain.Request{Domain: state.Domain, Operation: op, Actor: actor, RequestID: call + "-" + suffix, Timestamp: time.Now().Unix(), Parameters: p}
		if e := domain.Sign(&r, key); e != nil {
			t.Fatal(e)
		}
		return r
	}
	submit := func(r domain.Request) domain.Result {
		t.Helper()
		result, e := backend.Submit(r)
		if e != nil {
			t.Fatal(e)
		}
		return result
	}
	submit(req("identity.register", ownerID, "owner", domain.Parameters{IdentityID: ownerID, PublicKey: base64.StdEncoding.EncodeToString(pub), Kind: "user"}, owner))
	submit(req("identity.register", appID, "app", domain.Parameters{IdentityID: appID, PublicKey: base64.StdEncoding.EncodeToString(appPub), Kind: "application"}, app))
	dataRequest := req("data.register", ownerID, "data", domain.Parameters{DataID: dataID, Address: "https://example.org/data", DataSignature: base64.StdEncoding.EncodeToString(ed25519.Sign(owner, []byte("payload"))), Profile: &domain.DataProfile{Label: "live", Format: "text/plain", Description: "Fabric integration test"}}, owner)
	original := submit(dataRequest)
	if retry := submit(dataRequest); retry != original {
		t.Fatal("retry changed result")
	}
	conflicting := dataRequest
	conflicting.Parameters.Address = "https://example.org/other"
	domain.Sign(&conflicting, owner)
	if _, e = backend.Submit(conflicting); !errors.Is(e, domain.ErrConflict) {
		t.Fatalf("expected conflict, got %v", e)
	}
	forged := dataRequest
	forged.RequestID = "forged-" + suffix
	if _, e = backend.Submit(forged); !errors.Is(e, domain.ErrUnauthorized) {
		t.Fatalf("expected unauthorized, got %v", e)
	}
	permissions := []string{"Read"}
	expiry := time.Now().Add(time.Hour).Unix()
	submit(req("authorization.request", appID, "request", domain.Parameters{DataID: dataID, Recipient: appID, Permissions: permissions, ExpiresAt: expiry}, app))
	p := domain.Parameters{DataID: dataID, Recipient: appID, Status: domain.AuthorizationApproved}
	if _, e = backend.Submit(req("authorization.decide", appID, "wrong-owner", p, app)); !errors.Is(e, domain.ErrUnauthorized) {
		t.Fatalf("expected unauthorized owner, got %v", e)
	}
	decision := req("authorization.decide", ownerID, "approve", p, owner)
	accepted := submit(decision)
	got, e := backend.View()
	if e != nil {
		t.Fatal(e)
	}
	record := got.Authorizations[domain.AuthorizationKey(dataID, ownerID, appID)]
	direct, e := backend.GetAuthorization(dataID, ownerID, appID)
	if e != nil || !reflect.DeepEqual(direct, record) {
		t.Fatalf("direct authorization query: %v %v", direct, e)
	}
	if _, e = backend.GetIdentity("missing-" + suffix + ".uid"); !errors.Is(e, domain.ErrNotFound) {
		t.Fatalf("missing direct identity: %v", e)
	}
	if record.Status != domain.AuthorizationApproved || record.ExpiresAt != expiry || !reflect.DeepEqual(record.Permissions, permissions) {
		t.Fatal(record)
	}
	reopened, e := Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	if retry, e := reopened.Submit(decision); e != nil || retry != accepted {
		t.Fatalf("reconnect retry: %v %v", retry, e)
	}
	if got.Sequence != state.Sequence+5 {
		t.Fatal("retries or rejected requests changed state")
	}
	t.Logf("verified real Fabric commits, signature rejection, owner checks, request conflict and retry: sequence %d", got.Sequence)
}
