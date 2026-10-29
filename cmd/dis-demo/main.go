package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/han-ry19/DIS-reconstruct/internal/domain"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run() error {
	endpoint := flag.String("url", "http://127.0.0.1:8080", "DIS API URL")
	network := flag.String("domain", "dis-local", "Request domain")
	flag.Parse()
	var random [8]byte
	if _, e := rand.Read(random[:]); e != nil {
		return e
	}
	suffix := hex.EncodeToString(random[:])
	ownerID := "user-" + suffix + ".uid"
	appID := "app-" + suffix + ".uid"
	dataID := "data-" + suffix + ".data"
	ownerPub, owner, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		return e
	}
	appPub, app, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		return e
	}
	client := &http.Client{Timeout: 10 * time.Second}
	send := func(op, actor string, p domain.Parameters, key ed25519.PrivateKey) error {
		req := domain.Request{Domain: *network, Operation: op, Actor: actor, RequestID: strings.ReplaceAll(op, ".", "-") + "-" + suffix, Timestamp: time.Now().Unix(), Parameters: p}
		if e := domain.Sign(&req, key); e != nil {
			return e
		}
		b, e := json.Marshal(req)
		if e != nil {
			return e
		}
		response, e := client.Post(*endpoint+"/v1/transactions", "application/json", bytes.NewReader(b))
		if e != nil {
			return e
		}
		defer response.Body.Close()
		body, e := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		if e != nil {
			return e
		}
		if response.StatusCode != 200 {
			return fmt.Errorf("%s: %s %s", op, response.Status, body)
		}
		fmt.Printf("%s: %s", op, body)
		return nil
	}
	if e = send("identity.register", ownerID, domain.Parameters{IdentityID: ownerID, PublicKey: base64.StdEncoding.EncodeToString(ownerPub), Kind: "user"}, owner); e != nil {
		return e
	}
	if e = send("identity.register", appID, domain.Parameters{IdentityID: appID, PublicKey: base64.StdEncoding.EncodeToString(appPub), Kind: "application"}, app); e != nil {
		return e
	}
	signature := base64.StdEncoding.EncodeToString(ed25519.Sign(owner, []byte("Example data payload")))
	if e = send("data.register", ownerID, domain.Parameters{DataID: dataID, Address: "https://data.example/item", DataSignature: signature, Profile: &domain.DataProfile{Label: "example", Format: "text/plain", Description: "DIS workflow example"}}, owner); e != nil {
		return e
	}
	if e = send("authorization.request", appID, domain.Parameters{DataID: dataID, Recipient: appID, Permissions: []string{"Read"}, ExpiresAt: time.Now().Add(time.Hour).Unix()}, app); e != nil {
		return e
	}
	if e = send("authorization.decide", ownerID, domain.Parameters{DataID: dataID, Recipient: appID, Status: domain.AuthorizationApproved}, owner); e != nil {
		return e
	}
	response, e := client.Get(*endpoint + "/v1/authorizations?data_id=" + dataID + "&owner_id=" + ownerID + "&recipient=" + appID)
	if e != nil {
		return e
	}
	defer response.Body.Close()
	var record domain.AuthorizationRecord
	if e = json.NewDecoder(response.Body).Decode(&record); e != nil {
		return e
	}
	if response.StatusCode != 200 || record.Status != domain.AuthorizationApproved {
		return fmt.Errorf("unexpected authorization result")
	}
	fmt.Println("Authorization lookup:")
	return json.NewEncoder(os.Stdout).Encode(record)
}
