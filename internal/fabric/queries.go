package fabric

import (
	"encoding/json"
	"github.com/han-ry19/DIS-reconstruct/internal/domain"
	"strings"
)

func (b *Backend) GetIdentity(id string) (domain.Identity, error) {
	var v domain.Identity
	raw, e := b.contract.EvaluateTransaction("GetIdentity", id)
	if e != nil {
		if strings.Contains(e.Error(), "not found") {
			return v, domain.ErrNotFound
		}
		return v, e
	}
	e = json.Unmarshal(raw, &v)
	return v, e
}
func (b *Backend) GetData(id string) (domain.DataIdentifier, error) {
	var v domain.DataIdentifier
	raw, e := b.contract.EvaluateTransaction("GetData", id)
	if e != nil {
		if strings.Contains(e.Error(), "not found") {
			return v, domain.ErrNotFound
		}
		return v, e
	}
	e = json.Unmarshal(raw, &v)
	return v, e
}
func (b *Backend) GetAuthorization(data, recipient string) (domain.AuthorizationRecord, error) {
	var v domain.AuthorizationRecord
	raw, e := b.contract.EvaluateTransaction("GetAuthorization", data, recipient)
	if e != nil {
		if strings.Contains(e.Error(), "not found") {
			return v, domain.ErrNotFound
		}
		return v, e
	}
	e = json.Unmarshal(raw, &v)
	return v, e
}

// Domain reads deployment metadata without loading the business-state snapshot.
func (b *Backend) Domain() (string, error) {
	raw, err := b.contract.EvaluateTransaction("GetDomain")
	if err != nil {
		return "", err
	}
	var name string
	err = json.Unmarshal(raw, &name)
	return name, err
}
