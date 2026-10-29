package domain

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var (
	ErrInvalid      = errors.New("invalid request")
	ErrUnauthorized = errors.New("unauthorized")
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
	namePattern     = regexp.MustCompile(`^[a-z0-9]([a-z0-9.-]{0,126}[a-z0-9])?$`)
	requestPattern  = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
)

// SigningBytes defines the versioned request signature format. Signature is excluded.
func SigningBytes(r Request) ([]byte, error) {
	r.Signature = ""
	b, err := json.Marshal(r)
	return append([]byte("DIS-request-v1\n"), b...), err
}
func Sign(r *Request, key ed25519.PrivateKey) error {
	if len(key) != ed25519.PrivateKeySize {
		return ErrInvalid
	}
	b, err := SigningBytes(*r)
	if err != nil {
		return err
	}
	r.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(key, b))
	return nil
}
func AuthorizationKey(data, owner, recipient string) string {
	return data + "|" + owner + "|" + recipient
}
func validName(s string) bool {
	if !namePattern.MatchString(s) || strings.Contains(s, "..") {
		return false
	}
	for _, label := range strings.Split(s, ".") {
		if len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
	}
	return true
}
func decodeKey(s string) (ed25519.PublicKey, error) {
	b, e := base64.StdEncoding.DecodeString(s)
	if e != nil || len(b) != ed25519.PublicKeySize {
		return nil, ErrInvalid
	}
	return ed25519.PublicKey(b), nil
}

// Apply performs DIA management operations. now is supplied by the execution context:
// the server clock for development, or the shared transaction timestamp in Fabric.
func Apply(s *State, r Request, now int64) (Result, error) {
	fail := func(e error) (Result, error) { return Result{}, e }
	if r.Domain != s.Domain || !validName(r.Actor) || !requestPattern.MatchString(r.RequestID) {
		return fail(ErrInvalid)
	}
	p := r.Parameters
	var pub ed25519.PublicKey
	var err error
	if r.Operation == "identity.register" {
		pub, err = decodeKey(p.PublicKey)
	} else {
		id, ok := s.Identities[r.Actor]
		if !ok {
			return fail(ErrUnauthorized)
		}
		pub, err = decodeKey(id.PublicKey)
	}
	if err != nil {
		return fail(ErrInvalid)
	}
	b, err := SigningBytes(r)
	if err != nil {
		return fail(err)
	}
	sig, err := base64.StdEncoding.DecodeString(r.Signature)
	if err != nil || !ed25519.Verify(pub, b, sig) {
		return fail(ErrUnauthorized)
	}
	digest := sha256.Sum256(b)
	hash := hex.EncodeToString(digest[:])
	call := r.Actor + "|" + r.RequestID
	if old, ok := s.Receipts[call]; ok {
		if old.Digest != hash {
			return fail(ErrConflict)
		}
		return old.Result, nil
	}
	if r.Timestamp <= 0 || now <= 0 || r.Timestamp > now+300 || r.Timestamp < now-300 {
		return fail(fmt.Errorf("%w: timestamp outside execution window", ErrInvalid))
	}
	if s.Sequence == ^uint64(0) {
		return fail(ErrConflict)
	}
	object := ""
	switch r.Operation {
	case "identity.register":
		if !strings.HasSuffix(r.Actor, ".uid") || strings.Count(r.Actor, ".") != 1 || p.IdentityID != r.Actor || (p.Kind != "user" && p.Kind != "application") {
			return fail(ErrInvalid)
		}
		if _, ok := s.Identities[r.Actor]; ok {
			return fail(ErrConflict)
		}
		object = r.Actor
		s.Identities[object] = Identity{ID: object, PublicKey: p.PublicKey, Kind: p.Kind, CreatedAt: now}
	case "data.register":
		actor := s.Identities[r.Actor]
		u, e := url.Parse(p.Address)
		contentSig, se := base64.StdEncoding.DecodeString(p.DataSignature)
		if actor.Kind != "user" {
			return fail(ErrUnauthorized)
		}
		if !validName(p.DataID) || !strings.HasSuffix(p.DataID, ".data") || strings.Count(p.DataID, ".") != 1 || p.Profile == nil || e != nil || u.Scheme == "" || u.Host == "" || u.User != nil || se != nil || len(contentSig) != ed25519.SignatureSize {
			return fail(ErrInvalid)
		}
		if _, ok := s.Data[p.DataID]; ok {
			return fail(ErrConflict)
		}
		object = p.DataID
		s.Data[object] = DataIdentifier{ID: object, OwnerID: r.Actor, Address: p.Address, DataSig: p.DataSignature, Profile: *p.Profile, CreatedAt: now}
	case "authorization.request":
		data, ok := s.Data[p.DataID]
		if !ok {
			return fail(ErrNotFound)
		}
		if s.Identities[r.Actor].Kind != "application" || p.Recipient != r.Actor {
			return fail(ErrUnauthorized)
		}
		if len(p.Permissions) == 0 || p.ExpiresAt <= now {
			return fail(ErrInvalid)
		}
		seen := map[string]bool{}
		for _, v := range p.Permissions {
			if (v != "Create" && v != "Read" && v != "Update" && v != "Delete") || seen[v] {
				return fail(ErrInvalid)
			}
			seen[v] = true
		}
		object = AuthorizationKey(p.DataID, data.OwnerID, r.Actor)
		// DIA Algorithm 2 writes the requested pending record at its composite key.
		s.Authorizations[object] = AuthorizationRecord{DataID: p.DataID, OwnerID: data.OwnerID, Recipient: r.Actor, Status: AuthorizationPending, Permissions: append([]string(nil), p.Permissions...), ExpiresAt: p.ExpiresAt}
	case "authorization.decide":
		data, ok := s.Data[p.DataID]
		if !ok {
			return fail(ErrNotFound)
		}
		if data.OwnerID != r.Actor {
			return fail(ErrUnauthorized)
		}
		object = AuthorizationKey(p.DataID, r.Actor, p.Recipient)
		a, ok := s.Authorizations[object]
		if !ok {
			return fail(ErrNotFound)
		}
		if p.Status != AuthorizationApproved && p.Status != AuthorizationRejected {
			return fail(ErrInvalid)
		}
		a.Status = p.Status
		a.IssuedAt = now
		s.Authorizations[object] = a
	default:
		return fail(ErrInvalid)
	}
	s.Sequence++
	result := Result{RequestID: r.RequestID, Operation: r.Operation, ObjectID: object, Sequence: s.Sequence, CommittedAt: now}
	s.Receipts[call] = Receipt{Digest: hash, Result: result}
	return result, nil
}
