package fabric

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/han-ry19/DIS-reconstruct/internal/dns"
	"github.com/han-ry19/DIS-reconstruct/internal/domain"
	"os"
	"testing"
	"time"
)

func TestLiveFabricPendingTransferAndColdRecovery(t *testing.T) {
	path := os.Getenv("DIS_FABRIC_CONFIG")
	if path == "" {
		t.Skip("set DIS_FABRIC_CONFIG")
	}
	b, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	name, err := b.Domain()
	if err != nil {
		t.Fatal(err)
	}
	suffix := fmt.Sprintf("%x", time.Now().UnixNano())
	keys := map[string]ed25519.PrivateKey{}
	users := []string{"alice-" + suffix + ".uid", "bob-" + suffix + ".uid", "app-" + suffix + ".uid"}
	calls := 0
	send := func(op, actor string, p domain.Parameters) error {
		calls++
		r := domain.Request{Domain: name, Operation: op, Actor: actor, RequestID: fmt.Sprint(calls, "-", suffix), Timestamp: time.Now().Unix(), Parameters: p}
		if err := domain.Sign(&r, keys[actor]); err != nil {
			t.Fatal(err)
		}
		_, err := b.Submit(r)
		return err
	}
	must := func(op, actor string, p domain.Parameters) {
		t.Helper()
		if err := send(op, actor, p); err != nil {
			t.Fatal(err)
		}
	}
	for i, id := range users {
		pub, key, _ := ed25519.GenerateKey(rand.Reader)
		keys[id] = key
		kind := "user"
		if i == 2 {
			kind = "application"
		}
		must("identity.register", id, domain.Parameters{IdentityID: id, PublicKey: base64.StdEncoding.EncodeToString(pub), Kind: kind})
	}
	data, err := domain.NewDataID()
	if err != nil {
		t.Fatal(err)
	}
	must("data.register", users[0], domain.Parameters{DataID: data, Profile: &domain.DataProfile{Label: "transfer"}, Address: "https://example.org/pending", DataSignature: base64.StdEncoding.EncodeToString(ed25519.Sign(keys[users[0]], []byte("data")))})
	must("authorization.request", users[2], domain.Parameters{DataID: data, Recipient: users[2], Permissions: []string{"Read"}, ExpiresAt: time.Now().Unix() + 3600})
	must("data.update", users[0], domain.Parameters{DataID: data, Field: "RP", NewContent: users[1]})
	decision := domain.Parameters{DataID: data, Recipient: users[2], Status: domain.AuthorizationApproved}
	if err := send("authorization.decide", users[0], decision); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatal("former owner decision", err)
	}
	must("authorization.decide", users[1], decision)
	a, err := b.GetAuthorization(data, users[2])
	if err != nil || a.OwnerID != users[0] || a.Status != domain.AuthorizationApproved || a.IssuedAt <= 0 {
		t.Fatal(a, err)
	}
	current, err := b.View()
	if err != nil {
		t.Fatal(err)
	}
	recovered, height, err := b.Rebuild(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	current.Sequence = recovered.Sequence
	expected, _ := dns.Digest(current)
	actual, _ := dns.Digest(recovered)
	if expected != actual || height == 0 || len(recovered.Receipts) != 0 {
		t.Fatal("cold recovery differs from ledger", height)
	}
	t.Log("new owner approved pending request; valid-event replay matches current business state")
}
