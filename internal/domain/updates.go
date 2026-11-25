package domain

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strconv"
)

// updateData follows Dominate Algorithm 7: the current owner changes one RR value.
// RP changes ownership only; existing authorization TXT records are preserved.
func updateData(s *State, r Request) (string, error) {
	p := r.Parameters
	d, ok := s.Data[p.DataID]
	if !ok {
		return "", ErrNotFound
	}
	if d.OwnerID != r.Actor {
		return "", ErrUnauthorized
	}
	switch p.Field {
	case "URI":
		u, err := url.Parse(p.NewContent)
		if len(p.NewContent) > MaxAddressBytes || err != nil || u.Scheme == "" || u.Host == "" || u.User != nil {
			return "", ErrInvalid
		}
		d.Address = p.NewContent
	case "TXT":
		b, err := base64.StdEncoding.DecodeString(p.NewContent)
		if err != nil || len(b) != 64 {
			return "", ErrInvalid
		}
		d.DataSig = p.NewContent
	case "RP":
		owner, exists := s.Identities[p.NewContent]
		if !exists {
			return "", ErrNotFound
		}
		if owner.Kind != "user" {
			return "", ErrInvalid
		}
		d.OwnerID = owner.ID
	default:
		return "", ErrInvalid
	}
	s.Data[d.ID] = d
	return d.ID, nil
}

// updateAuthorization follows Algorithm 8. The current data owner may edit an
// existing authorization; changing recipient moves it rather than copying a grant.
func updateAuthorization(s *State, r Request, now int64) (string, error) {
	p := r.Parameters
	d, ok := s.Data[p.DataID]
	if !ok {
		return "", ErrNotFound
	}
	if d.OwnerID != r.Actor {
		return "", ErrUnauthorized
	}
	oldKey, a, ok := FindAuthorization(*s, d.ID, p.Recipient)
	if !ok {
		return "", ErrNotFound
	}
	switch p.Field {
	case "status":
		status := AuthorizationStatus(p.NewContent)
		if status != AuthorizationApproved && status != AuthorizationRejected {
			return "", ErrInvalid
		}
		if a.Status == AuthorizationPending {
			a.IssuedAt = now
		}
		a.Status = status
	case "permissions":
		var permissions []string
		if err := json.Unmarshal([]byte(p.NewContent), &permissions); err != nil || !validPermissions(permissions) {
			return "", ErrInvalid
		}
		a.Permissions = permissions
	case "recipient":
		id, exists := s.Identities[p.NewContent]
		if !exists {
			return "", ErrNotFound
		}
		if id.Kind != "application" {
			return "", ErrInvalid
		}
		for k, other := range s.Authorizations {
			if k != oldKey && other.DataID == d.ID && other.Recipient == id.ID {
				return "", ErrConflict
			}
		}
		a.Recipient = id.ID
	case "expires_at":
		expires, err := strconv.ParseInt(p.NewContent, 10, 64)
		if err != nil || expires <= 0 {
			return "", ErrInvalid
		}
		// A past positive expiration intentionally makes an existing grant expired.
		a.ExpiresAt = expires
	default:
		return "", ErrInvalid
	}
	newKey := AuthorizationKey(a.DataID, a.OwnerID, a.Recipient)
	delete(s.Authorizations, oldKey)
	s.Authorizations[newKey] = a
	return newKey, nil
}

func validPermissions(values []string) bool {
	if len(values) == 0 {
		return false
	}
	seen := map[string]bool{}
	for _, v := range values {
		if (v != "Create" && v != "Read" && v != "Update" && v != "Delete") || seen[v] {
			return false
		}
		seen[v] = true
	}
	return true
}

// FindAuthorization selects the unique public authorization name, independent of issuer.
func FindAuthorization(s State, data, recipient string) (string, AuthorizationRecord, bool) {
	for k, a := range s.Authorizations {
		if a.DataID == data && a.Recipient == recipient {
			return k, a, true
		}
	}
	return "", AuthorizationRecord{}, false
}
