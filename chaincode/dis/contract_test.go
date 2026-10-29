package dis

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"github.com/han-ry19/DIS-reconstruct/internal/domain"
	"github.com/hyperledger/fabric-chaincode-go/shimtest"
	"testing"
	"time"
)

func TestFabricAdapter(t *testing.T) {
	stub := shimtest.NewMockStub("dis", &Contract{})
	if r := stub.MockInit("init", [][]byte{[]byte("Init"), []byte("test")}); r.Status != 200 {
		t.Fatal(r.Message)
	}
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	req := domain.Request{Domain: "test", Operation: "identity.register", Actor: "alice.uid", RequestID: "register", Timestamp: time.Now().Unix(), Parameters: domain.Parameters{IdentityID: "alice.uid", PublicKey: base64.StdEncoding.EncodeToString(pub), Kind: "user"}}
	domain.Sign(&req, key)
	b, _ := json.Marshal(req)
	r := stub.MockInvoke("tx1", [][]byte{[]byte("Submit"), b})
	if r.Status != 200 {
		t.Fatal(r.Message)
	}
	var result domain.Result
	json.Unmarshal(r.Payload, &result)
	if result.Sequence != 1 {
		t.Fatal(result)
	}
	repeat := stub.MockInvoke("tx2", [][]byte{[]byte("Submit"), b})
	if repeat.Status != 200 || string(repeat.Payload) != string(r.Payload) {
		t.Fatal("retry changed result")
	}
	r = stub.MockInvoke("query", [][]byte{[]byte("GetIdentity"), []byte("alice.uid")})
	if r.Status != 200 {
		t.Fatal(r.Message)
	}
	var id domain.Identity
	json.Unmarshal(r.Payload, &id)
	if id.PublicKey != req.Parameters.PublicKey {
		t.Fatal(id)
	}
	req.RequestID = "forged"
	b, _ = json.Marshal(req)
	r = stub.MockInvoke("bad", [][]byte{[]byte("Submit"), b})
	if r.Status == 200 {
		t.Fatal("forged request accepted")
	}
	r = stub.MockInvoke("snapshot", [][]byte{[]byte("GetSnapshot")})
	var state domain.State
	json.Unmarshal(r.Payload, &state)
	if state.Sequence != 1 {
		t.Fatal("failed invoke changed ledger")
	}
	if r = stub.MockInit("again", [][]byte{[]byte("Init"), []byte("other")}); r.Status == 200 {
		t.Fatal("reinitialization accepted")
	}
}
