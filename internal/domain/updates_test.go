package domain

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

const updateNow int64 = 1800000000

func updateFixture() (State, map[string]ed25519.PrivateKey) {
	s := NewState("updates")
	keys := map[string]ed25519.PrivateKey{}
	for i, id := range []string{"alice.uid", "bob.uid", "app.uid", "other.uid"} {
		seed := make([]byte, 32)
		seed[0] = byte(i + 1)
		k := ed25519.NewKeyFromSeed(seed)
		keys[id] = k
		kind := "user"
		if i >= 2 {
			kind = "application"
		}
		s.Identities[id] = Identity{ID: id, Kind: kind, PublicKey: base64.StdEncoding.EncodeToString(k.Public().(ed25519.PublicKey))}
	}
	s.Data["e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data"] = DataIdentifier{ID: "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", OwnerID: "alice.uid", Address: "https://example.org/old", DataSig: base64.StdEncoding.EncodeToString(make([]byte, 64)), CreatedAt: 1}
	a := AuthorizationRecord{DataID: "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", OwnerID: "alice.uid", Recipient: "app.uid", Status: AuthorizationApproved, Permissions: []string{"Read"}, IssuedAt: 2, ExpiresAt: updateNow + 3600}
	s.Authorizations[AuthorizationKey(a.DataID, a.OwnerID, a.Recipient)] = a
	return s, keys
}
func signedUpdate(t *testing.T, keys map[string]ed25519.PrivateKey, op, actor, id string, p Parameters) Request {
	t.Helper()
	r := Request{Domain: "updates", Operation: op, Actor: actor, RequestID: id, Timestamp: updateNow, Parameters: p}
	if e := Sign(&r, keys[actor]); e != nil {
		t.Fatal(e)
	}
	return r
}
func TestUpdateFields(t *testing.T) {
	cases := []struct {
		op, field, value string
		check            func(State) bool
	}{
		{"data.update", "URI", "https://example.org/new", func(s State) bool {
			return s.Data["e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data"].Address == "https://example.org/new"
		}},
		{"data.update", "TXT", base64.StdEncoding.EncodeToString(make([]byte, 64)), func(s State) bool { return len(s.Data["e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data"].DataSig) == 88 }},
		{"data.update", "RP", "bob.uid", func(s State) bool { return s.Data["e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data"].OwnerID == "bob.uid" }},
		{"authorization.update", "status", "rejected", func(s State) bool {
			return s.Authorizations["e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data|alice.uid|app.uid"].Status == AuthorizationRejected
		}},
		{"authorization.update", "permissions", "[\"Read\",\"Update\"]", func(s State) bool {
			return reflect.DeepEqual(s.Authorizations["e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data|alice.uid|app.uid"].Permissions, []string{"Read", "Update"})
		}},
		{"authorization.update", "recipient", "other.uid", func(s State) bool {
			_, old := s.Authorizations["e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data|alice.uid|app.uid"]
			return !old && s.Authorizations["e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data|alice.uid|other.uid"].Recipient == "other.uid"
		}},
		{"authorization.update", "expires_at", "1799999999", func(s State) bool {
			return s.Authorizations["e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data|alice.uid|app.uid"].ExpiresAt == updateNow-1
		}},
	}
	for _, tc := range cases {
		t.Run(tc.op+"/"+tc.field, func(t *testing.T) {
			s, keys := updateFixture()
			r := signedUpdate(t, keys, tc.op, "alice.uid", "edit", Parameters{DataID: "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", Recipient: "app.uid", Field: tc.field, NewContent: tc.value})
			res, e := Apply(&s, r, updateNow)
			if e != nil || !tc.check(s) {
				t.Fatal(res, e, s)
			}
			if s.Data["e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data"].CreatedAt != 1 {
				t.Fatal("update changed registration time")
			}
			before, _ := json.Marshal(s)
			replay, e := Apply(&s, r, updateNow+1000)
			after, _ := json.Marshal(s)
			if e != nil || replay != res || string(before) != string(after) {
				t.Fatal("exact retry reapplied update", e)
			}
			r.Parameters.NewContent = "changed"
			if _, e = Apply(&s, r, updateNow); !errors.Is(e, ErrUnauthorized) {
				t.Fatal("unsigned field change accepted", e)
			}
		})
	}
}
func TestInvalidUpdatesLeaveStateUnchanged(t *testing.T) {
	cases := []struct {
		name, op, field, value, actor, data, recipient string
		want                                           error
	}{
		{"relative URI", "data.update", "URI", "/local", "alice.uid", "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", "app.uid", ErrInvalid},
		{"URI credentials", "data.update", "URI", "https://x:y@example.org", "alice.uid", "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", "app.uid", ErrInvalid},
		{"bad signature", "data.update", "TXT", "bad", "alice.uid", "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", "app.uid", ErrInvalid},
		{"unknown owner", "data.update", "RP", "missing.uid", "alice.uid", "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", "app.uid", ErrNotFound},
		{"app owner", "data.update", "RP", "app.uid", "alice.uid", "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", "app.uid", ErrInvalid},
		{"unknown data field", "data.update", "A", "x", "alice.uid", "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", "app.uid", ErrInvalid},
		{"other user", "data.update", "URI", "https://example.org/new", "bob.uid", "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", "app.uid", ErrUnauthorized},
		{"missing data", "data.update", "URI", "https://example.org/new", "alice.uid", "missing.data", "app.uid", ErrNotFound},
		{"missing auth", "authorization.update", "status", "rejected", "alice.uid", "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", "missing.uid", ErrNotFound},
		{"status", "authorization.update", "status", "pending", "alice.uid", "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", "app.uid", ErrInvalid},
		{"unknown field", "authorization.update", "unknown", "x", "alice.uid", "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", "app.uid", ErrInvalid},
		{"permissions JSON", "authorization.update", "permissions", "Read", "alice.uid", "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", "app.uid", ErrInvalid},
		{"empty permissions", "authorization.update", "permissions", "[]", "alice.uid", "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", "app.uid", ErrInvalid},
		{"duplicate permissions", "authorization.update", "permissions", "[\"Read\",\"Read\"]", "alice.uid", "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", "app.uid", ErrInvalid},
		{"unknown permissions", "authorization.update", "permissions", "[\"Root\"]", "alice.uid", "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", "app.uid", ErrInvalid},
		{"unknown recipient", "authorization.update", "recipient", "missing.uid", "alice.uid", "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", "app.uid", ErrNotFound},
		{"user recipient", "authorization.update", "recipient", "bob.uid", "alice.uid", "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", "app.uid", ErrInvalid},
		{"bad expiry", "authorization.update", "expires_at", "tomorrow", "alice.uid", "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", "app.uid", ErrInvalid},
		{"zero expiry", "authorization.update", "expires_at", "0", "alice.uid", "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", "app.uid", ErrInvalid},
		{"negative expiry", "authorization.update", "expires_at", "-1", "alice.uid", "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", "app.uid", ErrInvalid},
		{"overflow expiry", "authorization.update", "expires_at", "9223372036854775808", "alice.uid", "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", "app.uid", ErrInvalid},
		{"other auth actor", "authorization.update", "status", "rejected", "bob.uid", "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", "app.uid", ErrUnauthorized},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, keys := updateFixture()
			before, _ := json.Marshal(s)
			r := signedUpdate(t, keys, tc.op, tc.actor, "invalid", Parameters{DataID: tc.data, Recipient: tc.recipient, Field: tc.field, NewContent: tc.value})
			_, e := Apply(&s, r, updateNow)
			after, _ := json.Marshal(s)
			if !errors.Is(e, tc.want) || string(before) != string(after) {
				t.Fatalf("error=%v want=%v; changed=%t", e, tc.want, string(before) != string(after))
			}
		})
	}
}
func TestTransferPreservesGrantsAndChangesManager(t *testing.T) {
	s, keys := updateFixture()
	old := s.Authorizations["e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data|alice.uid|app.uid"]
	transfer := signedUpdate(t, keys, "data.update", "alice.uid", "transfer", Parameters{DataID: "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", Field: "RP", NewContent: "bob.uid"})
	if _, e := Apply(&s, transfer, updateNow); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(s.Authorizations["e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data|alice.uid|app.uid"], old) {
		t.Fatal("transfer modified existing grant")
	}
	p := Parameters{DataID: "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", Recipient: "app.uid", Field: "status", NewContent: "rejected"}
	if _, e := Apply(&s, signedUpdate(t, keys, "authorization.update", "alice.uid", "oldowner", p), updateNow); !errors.Is(e, ErrUnauthorized) {
		t.Fatal(e)
	}
	if _, e := Apply(&s, signedUpdate(t, keys, "authorization.update", "bob.uid", "newowner", p), updateNow); e != nil {
		t.Fatal(e)
	}
	// A request cannot introduce a second authorization at the same DNS name.
	p = Parameters{DataID: "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", Recipient: "app.uid", Permissions: []string{"Read"}, ExpiresAt: updateNow + 100}
	if _, e := Apply(&s, signedUpdate(t, keys, "authorization.request", "app.uid", "duplicate-issuer", p), updateNow); !errors.Is(e, ErrConflict) {
		t.Fatal(e)
	}
}
func TestRecipientCollisionAcrossIssuers(t *testing.T) {
	s, keys := updateFixture()
	other := s.Authorizations["e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data|alice.uid|app.uid"]
	other.OwnerID = "bob.uid"
	other.Recipient = "other.uid"
	other.Status = AuthorizationPending
	s.Authorizations[AuthorizationKey(other.DataID, other.OwnerID, other.Recipient)] = other
	before, _ := json.Marshal(s)
	r := signedUpdate(t, keys, "authorization.update", "alice.uid", "collision", Parameters{DataID: "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", Recipient: "app.uid", Field: "recipient", NewContent: "other.uid"})
	if _, e := Apply(&s, r, updateNow); !errors.Is(e, ErrConflict) {
		t.Fatal(e)
	}
	after, _ := json.Marshal(s)
	if string(before) != string(after) {
		t.Fatal("collision mutated state")
	}
}
