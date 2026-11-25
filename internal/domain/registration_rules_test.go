package domain

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestDataRegistrationUUIDContract(t *testing.T) {
	s, keys := updateFixture()
	id, err := NewDataID()
	if err != nil {
		t.Fatal(err)
	}
	if !ValidDataID(id) {
		t.Fatal("generator violated UUIDv4 contract", id)
	}
	// Use complete valid fields so these checks exercise identifier validation.
	p := Parameters{DataID: id, Address: "https://example.org", DataSignature: base64.StdEncoding.EncodeToString(make([]byte, 64)), Profile: &DataProfile{Label: "Readable name"}}
	for _, bad := range []string{"readable.data", id[:14] + "5" + id[15:], strings.ToUpper(id), id + ".", "00000000-0000-4000-0000-000000000000.data"} {
		p.DataID = bad
		before, _ := json.Marshal(s)
		if _, err := Apply(&s, signedUpdate(t, keys, "data.register", "alice.uid", "invalid-id", p), updateNow); !errors.Is(err, ErrInvalid) {
			t.Fatalf("accepted %q: %v", bad, err)
		}
		after, _ := json.Marshal(s)
		if string(before) != string(after) {
			t.Fatal("invalid identifier changed state")
		}
	}
	p.DataID = id
	r := signedUpdate(t, keys, "data.register", "alice.uid", "valid-id", p)
	result, err := Apply(&s, r, updateNow)
	if err != nil {
		t.Fatal(err)
	}
	if result.ObjectID != id {
		t.Fatal(result)
	}
	retry, err := Apply(&s, r, updateNow+1000)
	if err != nil || retry != result {
		t.Fatal("retry changed identifier", err)
	}
	if _, err := Apply(&s, signedUpdate(t, keys, "data.register", "alice.uid", "collision", p), updateNow); !errors.Is(err, ErrConflict) {
		t.Fatal("duplicate identifier accepted", err)
	}
}

func TestExistingAuthorizationRejectsReapplication(t *testing.T) {
	for _, tc := range []struct {
		name     string
		status   AuthorizationStatus
		expiry   int64
		transfer bool
	}{
		{"pending", AuthorizationPending, updateNow + 100, false},
		{"approved", AuthorizationApproved, updateNow + 100, false},
		{"rejected", AuthorizationRejected, updateNow + 100, false},
		{"expired", AuthorizationApproved, 1, false},
		{"transferred", AuthorizationApproved, updateNow + 100, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, keys := updateFixture()
			var a AuthorizationRecord
			for _, v := range s.Authorizations {
				a = v
			}
			a.Status = tc.status
			a.ExpiresAt = tc.expiry
			s.Authorizations[AuthorizationKey(a.DataID, a.OwnerID, a.Recipient)] = a
			if tc.transfer {
				d := s.Data[a.DataID]
				d.OwnerID = "bob.uid"
				s.Data[d.ID] = d
			}
			before, _ := json.Marshal(s)
			p := Parameters{DataID: a.DataID, Recipient: a.Recipient, Permissions: []string{"Read"}, ExpiresAt: updateNow + 3600}
			if _, err := Apply(&s, signedUpdate(t, keys, "authorization.request", a.Recipient, "repeat", p), updateNow); !errors.Is(err, ErrConflict) {
				t.Fatal(err)
			}
			after, _ := json.Marshal(s)
			if string(before) != string(after) {
				t.Fatal("reapplication changed state")
			}
		})
	}
}

func TestAuthorizationRequestRetryAfterOwnerDecision(t *testing.T) {
	s, keys := updateFixture()
	s.Authorizations = map[string]AuthorizationRecord{}
	var id string
	for k := range s.Data {
		id = k
	}
	r := signedUpdate(t, keys, "authorization.request", "app.uid", "request", Parameters{DataID: id, Recipient: "app.uid", Permissions: []string{"Read"}, ExpiresAt: updateNow + 3600})
	result, err := Apply(&s, r, updateNow)
	if err != nil {
		t.Fatal(err)
	}
	decision := signedUpdate(t, keys, "authorization.decide", "alice.uid", "decision", Parameters{DataID: id, Recipient: "app.uid", Status: AuthorizationApproved})
	if _, err := Apply(&s, decision, updateNow); err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(s)
	retry, err := Apply(&s, r, updateNow+1000)
	if err != nil || retry != result {
		t.Fatal("request retry failed", err)
	}
	after, _ := json.Marshal(s)
	if string(before) != string(after) {
		t.Fatal("request retry replaced decision")
	}
}

func TestLegacyDataIdentifiersRemainManageable(t *testing.T) {
	s, keys := updateFixture()
	var old string
	for id := range s.Data {
		old = id
	}
	d := s.Data[old]
	delete(s.Data, old)
	d.ID = "legacy.data"
	s.Data[d.ID] = d
	r := signedUpdate(t, keys, "data.update", "alice.uid", "legacy-update", Parameters{DataID: d.ID, Field: "URI", NewContent: "https://example.org/updated"})
	if _, err := Apply(&s, r, updateNow); err != nil {
		t.Fatal("legacy identifier cannot be updated", err)
	}
	if s.Data[d.ID].Address != "https://example.org/updated" {
		t.Fatal("legacy update missing")
	}
}
