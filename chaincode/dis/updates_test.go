package dis

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	projection "github.com/han-ry19/DIS-reconstruct/internal/dns"
	"github.com/han-ry19/DIS-reconstruct/internal/domain"
	"github.com/hyperledger/fabric-chaincode-go/shimtest"
)

type managementHarness struct {
	t         *testing.T
	stub      *shimtest.MockStub
	keys      map[string]ed25519.PrivateKey
	projected domain.State
	n         int
}

func newManagementHarness(t *testing.T) *managementHarness {
	t.Helper()
	h := &managementHarness{t: t, stub: shimtest.NewMockStub("dis", &Contract{}), keys: map[string]ed25519.PrivateKey{}, projected: domain.NewState("management")}
	if r := h.stub.MockInit("init", [][]byte{[]byte("Init"), []byte("management")}); r.Status != 200 {
		t.Fatal(r.Message)
	}
	for i, id := range []string{"alice.uid", "bob.uid", "app.uid", "other.uid"} {
		seed := make([]byte, 32)
		seed[0] = byte(i + 1)
		k := ed25519.NewKeyFromSeed(seed)
		h.keys[id] = k
		kind := "user"
		if i >= 2 {
			kind = "application"
		}
		h.submit("identity.register", id, domain.Parameters{IdentityID: id, Kind: kind, PublicKey: base64.StdEncoding.EncodeToString(k.Public().(ed25519.PublicKey))}, true)
	}
	h.submit("data.register", "alice.uid", domain.Parameters{DataID: "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", Address: "https://example.org/old", DataSignature: base64.StdEncoding.EncodeToString(make([]byte, 64)), Profile: &domain.DataProfile{Label: "Item"}}, true)
	return h
}
func (h *managementHarness) query(function string, args ...string) []byte {
	h.t.Helper()
	raw := [][]byte{[]byte(function)}
	for _, a := range args {
		raw = append(raw, []byte(a))
	}
	r := h.stub.MockInvoke("query", raw)
	if r.Status != 200 {
		h.t.Fatalf("%s: %s", function, r.Message)
	}
	return r.Payload
}
func (h *managementHarness) submit(op, actor string, p domain.Parameters, ok bool) domain.Result {
	h.t.Helper()
	h.n++
	r := domain.Request{Domain: "management", Actor: actor, Operation: op, RequestID: fmt.Sprintf("request-%d", h.n), Timestamp: time.Now().Unix(), Parameters: p}
	if e := domain.Sign(&r, h.keys[actor]); e != nil {
		h.t.Fatal(e)
	}
	b, _ := json.Marshal(r)
	before, _ := json.Marshal(h.stub.State)
	response := h.stub.MockInvoke(fmt.Sprintf("tx-%d", h.n), [][]byte{[]byte("Submit"), b})
	if !ok {
		after, _ := json.Marshal(h.stub.State)
		if response.Status == 200 || string(before) != string(after) {
			h.t.Fatalf("failed request changed ledger: %s", response.Message)
		}
		return domain.Result{}
	}
	if response.Status != 200 {
		h.t.Fatalf("%s: %s", op, response.Message)
	}
	var result domain.Result
	if e := json.Unmarshal(response.Payload, &result); e != nil {
		h.t.Fatal(e)
	}
	select {
	case event := <-h.stub.ChaincodeEventsChannel:
		var change domain.Change
		if e := json.Unmarshal(event.Payload, &change); e != nil {
			h.t.Fatal(e)
		}
		change.Sequence = h.projected.Sequence + 1
		if e := projection.ApplyChange(&h.projected, change); e != nil {
			h.t.Fatal(e)
		}
	default:
		h.t.Fatal("committed operation missing event")
	}
	var state domain.State
	if e := json.Unmarshal(h.query("GetSnapshot"), &state); e != nil {
		h.t.Fatal(e)
	}
	state.Sequence = h.projected.Sequence
	a, _ := projection.Digest(state)
	bhash, _ := projection.Digest(h.projected)
	if a != bhash {
		h.t.Fatalf("event projection differs from snapshot after %s: snapshot=%+v projection=%+v", op, state, h.projected)
	}
	// A transport retry must neither update transaction information nor emit events.
	beforeRetry, _ := json.Marshal(h.stub.State)
	retry := h.stub.MockInvoke("retry", [][]byte{[]byte("Submit"), b})
	afterRetry, _ := json.Marshal(h.stub.State)
	if retry.Status != 200 || string(response.Payload) != string(retry.Payload) || string(beforeRetry) != string(afterRetry) {
		h.t.Fatal("retry rewrote result or ledger")
	}
	select {
	case <-h.stub.ChaincodeEventsChannel:
		h.t.Fatal("retry emitted an event")
	default:
	}
	return result
}
func (h *managementHarness) grant() {
	h.t.Helper()
	h.submit("authorization.request", "app.uid", domain.Parameters{DataID: "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", Recipient: "app.uid", Permissions: []string{"Read"}, ExpiresAt: time.Now().Unix() + 3600}, true)
	h.submit("authorization.decide", "alice.uid", domain.Parameters{DataID: "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", Recipient: "app.uid", Status: domain.AuthorizationApproved}, true)
}
func TestManagementLedgerAndIndexes(t *testing.T) {
	h := newManagementHarness(t)
	h.grant()
	var index, id string
	json.Unmarshal(h.query("TransDOIToIndex", "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data"), &index)
	json.Unmarshal(h.query("TransIndexToDOI", index), &id)
	if id != "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data" {
		t.Fatal("mapping not reversible")
	}
	expected := base64.StdEncoding.EncodeToString([]byte("tx-5e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data"))
	if index != expected {
		t.Fatal("unexpected historical index format", index)
	}
	var original transactionInfo
	json.Unmarshal(h.query("GetTransactionInfo", "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data"), &original)
	var profile domain.DataProfile
	found, e := read(h.stub, registrationKind, &profile, index)
	if e != nil || !found || profile.Label != "Item" {
		t.Fatal("registration data missing", e)
	}
	h.submit("data.update", "alice.uid", domain.Parameters{DataID: "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", Field: "URI", NewContent: "https://example.org/new"}, true)
	h.submit("data.update", "alice.uid", domain.Parameters{DataID: "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", Field: "TXT", NewContent: base64.StdEncoding.EncodeToString(ed25519.Sign(h.keys["alice.uid"], []byte("changed payload")))}, true)
	h.submit("authorization.update", "alice.uid", domain.Parameters{DataID: "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", Recipient: "app.uid", Field: "permissions", NewContent: "[\"Read\",\"Update\"]"}, true)
	h.submit("authorization.update", "alice.uid", domain.Parameters{DataID: "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", Recipient: "app.uid", Field: "recipient", NewContent: "other.uid"}, true)
	old := h.stub.MockInvoke("old", [][]byte{[]byte("GetAuthorization"), []byte("e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data"), []byte("alice.uid"), []byte("app.uid")})
	if old.Status == 200 {
		t.Fatal("old recipient still in ledger")
	}
	var changed domain.AuthorizationRecord
	json.Unmarshal(h.query("GetAuthorization", "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", "alice.uid", "other.uid"), &changed)
	if !reflect.DeepEqual(changed.Permissions, []string{"Read", "Update"}) {
		t.Fatal(changed)
	}
	appIndex, _ := rawIndex(h.stub, doiToIndexKind, "app.uid")
	otherIndex, _ := rawIndex(h.stub, doiToIndexKind, "other.uid")
	oldKey, _ := key(h.stub, authorizedIndexKind, appIndex, index)
	newKey, _ := key(h.stub, authorizedIndexKind, otherIndex, index)
	if len(h.stub.State[oldKey]) != 0 || string(h.stub.State[newKey]) != index {
		t.Fatal("recipient index did not move")
	}
	h.submit("data.update", "alice.uid", domain.Parameters{DataID: "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", Field: "RP", NewContent: "bob.uid"}, true)
	aliceIndex, _ := rawIndex(h.stub, doiToIndexKind, "alice.uid")
	bobIndex, _ := rawIndex(h.stub, doiToIndexKind, "bob.uid")
	for _, kind := range []string{ownerIndexKind, authorizedIndexKind} {
		oldKey, _ := key(h.stub, kind, aliceIndex, index)
		newKey, _ := key(h.stub, kind, bobIndex, index)
		if len(h.stub.State[oldKey]) != 0 || string(h.stub.State[newKey]) != index {
			t.Fatal("ownership index did not move", kind)
		}
	}
	p := domain.Parameters{DataID: "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", Recipient: "other.uid", Field: "status", NewContent: "rejected"}
	h.submit("authorization.update", "alice.uid", p, false)
	h.submit("authorization.update", "bob.uid", p, true)
	var listing []domain.DataIdentifier
	json.Unmarshal(h.query("GetAuthorizedData", "other.uid"), &listing)
	if len(listing) != 0 {
		t.Fatal("revoked authorization listed as effective")
	}
	p.Field = "status"
	p.NewContent = "approved"
	h.submit("authorization.update", "bob.uid", p, true)
	json.Unmarshal(h.query("GetAuthorizedData", "other.uid"), &listing)
	if len(listing) != 1 {
		t.Fatal("approved authorization missing")
	}
	p.Field = "expires_at"
	p.NewContent = "1"
	h.submit("authorization.update", "bob.uid", p, true)
	json.Unmarshal(h.query("GetAuthorizedData", "other.uid"), &listing)
	if len(listing) != 0 {
		t.Fatal("expired authorization listed as effective")
	}
	json.Unmarshal(h.query("GetOwnedData", "alice.uid"), &listing)
	if len(listing) != 0 {
		t.Fatal("old owner retained ownership")
	}
	json.Unmarshal(h.query("GetOwnedData", "bob.uid"), &listing)
	if len(listing) != 1 {
		t.Fatal("new owner missing data")
	}
	var current transactionInfo
	json.Unmarshal(h.query("GetTransactionInfo", "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data"), &current)
	if current.CreatedAt != original.CreatedAt || current.Index != original.Index || current.TransactionID == original.TransactionID {
		t.Fatal("transaction history metadata incorrect", original, current)
	}
}
func TestPendingUpdateAndCollision(t *testing.T) {
	h := newManagementHarness(t)
	h.submit("authorization.request", "app.uid", domain.Parameters{DataID: "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", Recipient: "app.uid", Permissions: []string{"Read"}, ExpiresAt: time.Now().Unix() + 3600}, true)
	h.submit("authorization.update", "alice.uid", domain.Parameters{DataID: "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", Recipient: "app.uid", Field: "recipient", NewContent: "other.uid"}, true)
	var a domain.AuthorizationRecord
	json.Unmarshal(h.query("GetAuthorization", "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", "alice.uid", "other.uid"), &a)
	if a.Status != domain.AuthorizationPending {
		t.Fatal(a)
	}
	h.submit("authorization.decide", "alice.uid", domain.Parameters{DataID: "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", Recipient: "other.uid", Status: domain.AuthorizationApproved}, true)
	h.submit("authorization.request", "app.uid", domain.Parameters{DataID: "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", Recipient: "app.uid", Permissions: []string{"Read"}, ExpiresAt: time.Now().Unix() + 3600}, true)
	h.submit("authorization.update", "alice.uid", domain.Parameters{DataID: "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", Recipient: "other.uid", Field: "recipient", NewContent: "app.uid"}, false)
	// Moving back removes the pending key too; no old pending entry survives.
	h.submit("authorization.update", "alice.uid", domain.Parameters{DataID: "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", Recipient: "app.uid", Field: "status", NewContent: "rejected"}, true)
	json.Unmarshal(h.query("GetAuthorization", "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", "alice.uid", "app.uid"), &a)
	if a.Status != domain.AuthorizationRejected || a.IssuedAt == 0 {
		t.Fatal("pending status update missing decision time", a)
	}
}
func TestPendingCollisionFromHistoricalIssuer(t *testing.T) {
	h := newManagementHarness(t)
	h.submit("authorization.request", "other.uid", domain.Parameters{DataID: "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", Recipient: "other.uid", Permissions: []string{"Read"}, ExpiresAt: time.Now().Unix() + 3600}, true)
	h.submit("data.update", "alice.uid", domain.Parameters{DataID: "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", Field: "RP", NewContent: "bob.uid"}, true)
	h.submit("authorization.request", "app.uid", domain.Parameters{DataID: "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", Recipient: "app.uid", Permissions: []string{"Read"}, ExpiresAt: time.Now().Unix() + 3600}, true)
	h.submit("authorization.update", "bob.uid", domain.Parameters{DataID: "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", Recipient: "app.uid", Field: "recipient", NewContent: "other.uid"}, false)
	h.submit("authorization.request", "other.uid", domain.Parameters{DataID: "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", Recipient: "other.uid", Permissions: []string{"Read"}, ExpiresAt: time.Now().Unix() + 3600}, false)
}

func TestPendingDecisionAfterOwnershipTransfer(t *testing.T) {
	for _, viaUpdate := range []bool{false, true} {
		t.Run(fmt.Sprint(viaUpdate), func(t *testing.T) {
			h := newManagementHarness(t)
			data := "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data"
			h.submit("authorization.request", "app.uid", domain.Parameters{DataID: data, Recipient: "app.uid", Permissions: []string{"Read"}, ExpiresAt: time.Now().Unix() + 3600}, true)
			h.submit("data.update", "alice.uid", domain.Parameters{DataID: data, Field: "RP", NewContent: "bob.uid"}, true)
			op := "authorization.decide"
			params := domain.Parameters{DataID: data, Recipient: "app.uid", Status: domain.AuthorizationApproved}
			if viaUpdate {
				op = "authorization.update"
				params.Status = ""
				params.Field = "status"
				params.NewContent = "approved"
			}
			h.submit(op, "alice.uid", params, false)
			h.submit(op, "bob.uid", params, true)
			var a domain.AuthorizationRecord
			json.Unmarshal(h.query("GetAuthorization", data, "alice.uid", "app.uid"), &a)
			if a.Status != domain.AuthorizationApproved || a.IssuedAt <= 0 || a.OwnerID != "alice.uid" {
				t.Fatal(a)
			}
		})
	}
}

func TestSnapshotDoesNotReadOrReturnReceipts(t *testing.T) {
	h := newManagementHarness(t)
	before := h.query("GetSnapshot")
	// Malformed receipts would fail a scan; snapshot must not visit them.
	h.stub.MockTransactionStart("receipt-seed")
	for i := 0; i < 1000; i++ {
		k, _ := key(h.stub, receiptKind, "unused.uid", fmt.Sprint(i))
		h.stub.PutState(k, []byte("not-json"))
	}
	h.stub.MockTransactionEnd("receipt-seed")
	after := h.query("GetSnapshot")
	if string(before) != string(after) {
		t.Fatal("snapshot changed with retry history")
	}
}
