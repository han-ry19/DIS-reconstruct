package fabric

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/han-ry19/DIS-reconstruct/internal/domain"
	"github.com/hyperledger/fabric-gateway/pkg/client"
	peer "github.com/hyperledger/fabric-protos-go-apiv2/peer"
	"os"
	"testing"
	"time"
)

// Endorse every transaction before any commits so simulations share one state.
func TestLiveFabricIndependentAuthorizations(t *testing.T) {
	path := os.Getenv("DIS_FABRIC_CONFIG")
	if path == "" {
		t.Skip("set DIS_FABRIC_CONFIG for live validation")
	}
	backend, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	initial, err := backend.View()
	if err != nil {
		t.Fatal(err)
	}
	var random [8]byte
	if _, err = rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	suffix := hex.EncodeToString(random[:])
	owner := "owner-" + suffix + ".uid"
	next := "next-" + suffix + ".uid"
	apps := []string{"a-" + suffix + ".uid", "b-" + suffix + ".uid", "c-" + suffix + ".uid", "d-" + suffix + ".uid"}
	keys := map[string]ed25519.PrivateKey{}
	calls := 0
	request := func(op, actor string, p domain.Parameters) domain.Request {
		calls++
		r := domain.Request{Domain: initial.Domain, Operation: op, Actor: actor, RequestID: fmt.Sprintf("concurrency-%s-%d", suffix, calls), Timestamp: time.Now().Unix(), Parameters: p}
		if err := domain.Sign(&r, keys[actor]); err != nil {
			t.Fatal(err)
		}
		return r
	}
	submit := func(r domain.Request) {
		t.Helper()
		if _, err := backend.Submit(r); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range append([]string{owner, next}, apps...) {
		pub, k, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		keys[id] = k
		kind := "application"
		if id == owner || id == next {
			kind = "user"
		}
		submit(request("identity.register", id, domain.Parameters{IdentityID: id, Kind: kind, PublicKey: base64.StdEncoding.EncodeToString(pub)}))
	}
	data, idErr := domain.NewDataID()
	if idErr != nil {
		t.Fatal(idErr)
	}
	submit(request("data.register", owner, domain.Parameters{DataID: data, Address: "https://example.org/data", DataSignature: base64.StdEncoding.EncodeToString(ed25519.Sign(keys[owner], []byte("payload"))), Profile: &domain.DataProfile{Label: "Concurrency"}}))
	endorse := func(r domain.Request) *client.Transaction {
		t.Helper()
		raw, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		p, err := backend.contract.NewProposal("Submit", client.WithArguments(string(raw)))
		if err != nil {
			t.Fatal(err)
		}
		tx, err := p.Endorse()
		if err != nil {
			t.Fatal(err)
		}
		return tx
	}
	commit := func(tx *client.Transaction, want peer.TxValidationCode) {
		t.Helper()
		c, err := tx.Submit()
		if err != nil {
			t.Fatal(err)
		}
		status, err := c.Status()
		if err != nil {
			t.Fatal(err)
		}
		if status.Code != want {
			t.Fatalf("transaction %s: got %s, want %s", tx.TransactionID(), status.Code, want)
		}
	}
	batch := func(label string, rs []domain.Request) {
		t.Helper()
		txs := make([]*client.Transaction, len(rs))
		for i, r := range rs {
			txs[i] = endorse(r)
		}
		for _, tx := range txs {
			commit(tx, peer.TxValidationCode_VALID)
		}
		t.Logf("%s: %d transactions endorsed at the same state, all valid", label, len(txs))
	}
	var rs []domain.Request
	for _, app := range apps {
		rs = append(rs, request("authorization.request", app, domain.Parameters{DataID: data, Recipient: app, Permissions: []string{"Read"}, ExpiresAt: time.Now().Unix() + 3600}))
	}
	batch("requests", rs)
	rs = nil
	for _, app := range apps {
		rs = append(rs, request("authorization.decide", owner, domain.Parameters{DataID: data, Recipient: app, Status: domain.AuthorizationApproved}))
	}
	batch("decisions", rs)
	rs = nil
	for _, app := range apps {
		rs = append(rs, request("authorization.update", owner, domain.Parameters{DataID: data, Recipient: app, Field: "permissions", NewContent: "[\"Read\",\"Update\"]"}))
	}
	batch("updates", rs)
	a := endorse(request("authorization.update", owner, domain.Parameters{DataID: data, Recipient: apps[0], Field: "status", NewContent: "rejected"}))
	b := endorse(request("authorization.update", owner, domain.Parameters{DataID: data, Recipient: apps[0], Field: "expires_at", NewContent: "1"}))
	commit(a, peer.TxValidationCode_VALID)
	commit(b, peer.TxValidationCode_MVCC_READ_CONFLICT)
	pending := endorse(request("authorization.update", owner, domain.Parameters{DataID: data, Recipient: apps[1], Field: "status", NewContent: "rejected"}))
	transfer := endorse(request("data.update", owner, domain.Parameters{DataID: data, Field: "RP", NewContent: next}))
	commit(transfer, peer.TxValidationCode_VALID)
	commit(pending, peer.TxValidationCode_MVCC_READ_CONFLICT)
	final, err := backend.View()
	if err != nil {
		t.Fatal(err)
	}
	for i, app := range apps {
		auth, ok := final.Authorizations[domain.AuthorizationKey(data, owner, app)]
		if !ok || len(auth.Permissions) != 2 {
			t.Fatalf("lost authorization: %+v", auth)
		}
		want := domain.AuthorizationApproved
		if i == 0 {
			want = domain.AuthorizationRejected
		}
		if auth.Status != want {
			t.Fatalf("invalid transaction changed grant: %+v", auth)
		}
	}
	t.Log("same-grant and ownership conflicts retained; all four grants preserved")
}
