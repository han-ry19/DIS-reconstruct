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
func (b *Backend) GetAuthorization(data, owner, recipient string) (domain.AuthorizationRecord, error) {
	var v domain.AuthorizationRecord
	raw, e := b.contract.EvaluateTransaction("GetAuthorization", data, owner, recipient)
	if e != nil {
		if strings.Contains(e.Error(), "not found") {
			return v, domain.ErrNotFound
		}
		return v, e
	}
	e = json.Unmarshal(raw, &v)
	return v, e
}
