package dis

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/han-ry19/DIS-reconstruct/internal/domain"
	"github.com/hyperledger/fabric-chaincode-go/shim"
	"github.com/hyperledger/fabric-chaincode-go/shimtest"
)

type simulatedStub struct {
	shim.ChaincodeStubInterface
	state   map[string][]byte
	reads   map[string][]byte
	writes  map[string][]byte
	request []byte
}

func (s *simulatedStub) GetFunctionAndParameters() (string, []string) {
	return "Submit", []string{string(s.request)}
}
func (s *simulatedStub) GetState(k string) ([]byte, error) {
	v := s.state[k]
	s.reads[k] = bytes.Clone(v)
	return bytes.Clone(v), nil
}
func (s *simulatedStub) PutState(k string, v []byte) error { s.writes[k] = bytes.Clone(v); return nil }
func (s *simulatedStub) DelState(k string) error           { s.writes[k] = nil; return nil }
func (s *simulatedStub) SetEvent(string, []byte) error     { return nil }
func (s *simulatedStub) GetStateByPartialCompositeKey(string, []string) (shim.StateQueryIteratorInterface, error) {
	return nil, fmt.Errorf("unexpected range read on management path")
}
func simulateRequest(t *testing.T, h *managementHarness, id, op, actor string, p domain.Parameters) *simulatedStub {
	t.Helper()
	r := domain.Request{Domain: "management", Operation: op, Actor: actor, RequestID: id, Timestamp: time.Now().Unix(), Parameters: p}
	if err := domain.Sign(&r, h.keys[actor]); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(r)
	clock := shimtest.NewMockStub("simulation", &Contract{})
	clock.MockTransactionStart(id)
	s := &simulatedStub{ChaincodeStubInterface: clock, state: h.stub.State, reads: map[string][]byte{}, writes: map[string][]byte{}, request: raw}
	response := (&Contract{}).Invoke(s)
	if response.Status != 200 {
		t.Fatal(op, response.Message)
	}
	return s
}
func conflictsAfter(first, second *simulatedStub) bool {
	for k, v := range first.writes {
		if old, ok := second.reads[k]; ok && !bytes.Equal(old, v) {
			return true
		}
	}
	return false
}
func assertIndependent(t *testing.T, a, b *simulatedStub) {
	t.Helper()
	if conflictsAfter(a, b) || conflictsAfter(b, a) {
		t.Fatal("independent authorizations share a read/write dependency")
	}
	for k := range a.writes {
		if _, ok := b.writes[k]; ok {
			t.Fatalf("shared write: %q", k)
		}
	}
}
func TestIndependentAuthorizationReadWriteSets(t *testing.T) {
	h := newManagementHarness(t)
	request := func(id string) domain.Parameters {
		return domain.Parameters{DataID: "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", Recipient: id, Permissions: []string{"Read"}, ExpiresAt: time.Now().Unix() + 3600}
	}
	a := simulateRequest(t, h, "request-a", "authorization.request", "app.uid", request("app.uid"))
	b := simulateRequest(t, h, "request-b", "authorization.request", "other.uid", request("other.uid"))
	assertIndependent(t, a, b)
	h.submit("authorization.request", "app.uid", request("app.uid"), true)
	h.submit("authorization.request", "other.uid", request("other.uid"), true)
	decision := func(id string) domain.Parameters {
		return domain.Parameters{DataID: "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", Recipient: id, Status: domain.AuthorizationApproved}
	}
	a = simulateRequest(t, h, "decision-a", "authorization.decide", "alice.uid", decision("app.uid"))
	b = simulateRequest(t, h, "decision-b", "authorization.decide", "alice.uid", decision("other.uid"))
	assertIndependent(t, a, b)
	h.submit("authorization.decide", "alice.uid", decision("app.uid"), true)
	h.submit("authorization.decide", "alice.uid", decision("other.uid"), true)
	update := func(id, field, value string) domain.Parameters {
		return domain.Parameters{DataID: "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", Recipient: id, Field: field, NewContent: value}
	}
	a = simulateRequest(t, h, "update-a", "authorization.update", "alice.uid", update("app.uid", "status", "rejected"))
	b = simulateRequest(t, h, "update-b", "authorization.update", "alice.uid", update("other.uid", "permissions", "["+"\"Read\",\"Update\""+"]"))
	assertIndependent(t, a, b)
	same := simulateRequest(t, h, "same-grant", "authorization.update", "alice.uid", update("app.uid", "permissions", "[\"Read\",\"Update\"]"))
	if !conflictsAfter(a, same) {
		t.Fatal("same authorization lost conflict protection")
	}
	transfer := simulateRequest(t, h, "transfer", "data.update", "alice.uid", domain.Parameters{DataID: "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", Field: "RP", NewContent: "bob.uid"})
	if !conflictsAfter(transfer, a) {
		t.Fatal("ownership transfer lost conflict protection")
	}
}

func TestConcurrentRecipientMovesConflict(t *testing.T) {
	h := newManagementHarness(t)
	for _, app := range []string{"app.uid", "other.uid"} {
		h.submit("authorization.request", app, domain.Parameters{DataID: "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", Recipient: app, Permissions: []string{"Read"}, ExpiresAt: time.Now().Unix() + 3600}, true)
		h.submit("authorization.decide", "alice.uid", domain.Parameters{DataID: "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", Recipient: app, Status: domain.AuthorizationApproved}, true)
	}
	h.keys["target.uid"] = h.keys["app.uid"]
	h.submit("identity.register", "target.uid", domain.Parameters{IdentityID: "target.uid", Kind: "application", PublicKey: base64.StdEncoding.EncodeToString(h.keys["target.uid"].Public().(ed25519.PublicKey))}, true)
	a := simulateRequest(t, h, "move-a", "authorization.update", "alice.uid", domain.Parameters{DataID: "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", Recipient: "app.uid", Field: "recipient", NewContent: "target.uid"})
	b := simulateRequest(t, h, "move-b", "authorization.update", "alice.uid", domain.Parameters{DataID: "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", Recipient: "other.uid", Field: "recipient", NewContent: "target.uid"})
	if !conflictsAfter(a, b) || !conflictsAfter(b, a) {
		t.Fatal("recipient collision lost protection")
	}
}
