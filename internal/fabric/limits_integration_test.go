package fabric

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/han-ry19/DIS-reconstruct/internal/domain"
)

func TestLiveFabricRejectsOversizedAddress(t *testing.T) {
	path := os.Getenv("DIS_FABRIC_CONFIG")
	if path == "" {
		t.Skip("set DIS_FABRIC_CONFIG for live validation")
	}
	b, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	s, err := b.View()
	if err != nil {
		t.Fatal(err)
	}
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	id := "limits-" + hex.EncodeToString(pub[:8]) + ".uid"
	send := func(op, request string, p domain.Parameters) error {
		r := domain.Request{Domain: s.Domain, Operation: op, Actor: id, RequestID: request, Timestamp: time.Now().Unix(), Parameters: p}
		if err := domain.Sign(&r, key); err != nil {
			t.Fatal(err)
		}
		_, err := b.Submit(r)
		return err
	}
	if err = send("identity.register", "register", domain.Parameters{IdentityID: id, Kind: "user", PublicKey: base64.StdEncoding.EncodeToString(pub)}); err != nil {
		t.Fatal(err)
	}
	dataID, idErr := domain.NewDataID()
	if idErr != nil {
		t.Fatal(idErr)
	}
	p := domain.Parameters{DataID: dataID, Address: "https://example.org/" + strings.Repeat("x", 70000), DataSignature: base64.StdEncoding.EncodeToString(ed25519.Sign(key, []byte("data"))), Profile: &domain.DataProfile{}}
	if err = send("data.register", "oversized", p); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("oversized registration: %v", err)
	}
	if _, err = b.GetData(dataID); err == nil {
		t.Fatal("rejected data exists")
	}
	p.Address = "https://example.org/" + strings.Repeat("x", domain.MaxAddressBytes-len("https://example.org/"))
	if err = send("data.register", "accepted", p); err != nil {
		t.Fatal(err)
	}
	if err = send("data.update", "oversized-update", domain.Parameters{DataID: dataID, Field: "URI", NewContent: "https://example.org/" + strings.Repeat("x", 70000)}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("oversized update: %v", err)
	}
	data, err := b.GetData(dataID)
	if err != nil || data.Address != p.Address {
		t.Fatal("rejected update changed data", err)
	}
}
