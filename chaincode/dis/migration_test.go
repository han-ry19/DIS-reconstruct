package dis

import (
	"encoding/json"
	"github.com/han-ry19/DIS-reconstruct/internal/domain"
	"github.com/hyperledger/fabric-chaincode-go/shimtest"
	"testing"
)

func TestMigrateLegacyObjectsAndReceipts(t *testing.T) {
	stub := shimtest.NewMockStub("dis", &Contract{})
	s := domain.NewState("test")
	s.Sequence = 9
	s.Identities["alice.uid"] = domain.Identity{ID: "alice.uid"}
	s.Data["sample.data"] = domain.DataIdentifier{ID: "sample.data", OwnerID: "alice.uid"}
	approved := domain.AuthorizationRecord{DataID: "sample.data", OwnerID: "alice.uid", Recipient: "app.uid", Status: domain.AuthorizationApproved}
	pending := approved
	pending.Recipient = "pending.uid"
	pending.Status = domain.AuthorizationPending
	s.Authorizations[domain.AuthorizationKey(approved.DataID, approved.OwnerID, approved.Recipient)] = approved
	s.Authorizations[domain.AuthorizationKey(pending.DataID, pending.OwnerID, pending.Recipient)] = pending
	s.Receipts["alice.uid|call"] = domain.Receipt{Digest: "original", Result: domain.Result{Sequence: 9}}
	b, _ := json.Marshal(s)
	stub.MockTransactionStart("seed")
	stub.PutState(legacyKey, b)
	stub.MockTransactionEnd("seed")
	if r := stub.MockInvoke("migrate", [][]byte{[]byte("MigrateState")}); r.Status != 200 {
		t.Fatal(r.Message)
	}
	if b, _ := stub.GetState(legacyKey); b != nil {
		t.Fatal("legacy document retained")
	}
	r := stub.MockInvoke("view", [][]byte{[]byte("GetSnapshot")})
	if r.Status != 200 {
		t.Fatal(r.Message)
	}
	var got domain.State
	json.Unmarshal(r.Payload, &got)
	if got.Sequence != 9 || len(got.Data) != 1 || len(got.Authorizations) != 2 {
		t.Fatal(got)
	}
	d, ok, e := object(stub, "sample.data")
	if e != nil || !ok || len(d.Authorizations) != 0 {
		t.Fatalf("authorization still embedded: %v %v", d, e)
	}
	var migrated domain.AuthorizationRecord
	if ok, err := read(stub, authorizationKind, &migrated, "sample.data", "app.uid"); err != nil || !ok || migrated.Status != domain.AuthorizationApproved {
		t.Fatal("missing independent authorization", err)
	}
	m, err := getMetadata(stub)
	if err != nil || !m.IndependentAuthorizations || !m.ReceiptSequence {
		t.Fatal("incomplete migration", m, err)
	}
	var receipt domain.Receipt
	if ok, e := read(stub, receiptKind, &receipt, "alice.uid", "call"); e != nil || !ok || receipt.Digest != "original" {
		t.Fatal(receipt, e)
	}
	if r = stub.MockInvoke("again", [][]byte{[]byte("MigrateState")}); r.Status != 200 {
		t.Fatal(r.Message)
	}
}
