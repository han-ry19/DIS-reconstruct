package dis

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/han-ry19/DIS-reconstruct/internal/domain"
	"github.com/hyperledger/fabric-chaincode-go/shim"
)

const doiToIndexKind = "DOIToIndex"
const indexToDOIKind = "IndexToDOI"
const ownerIndexKind = "dataOwner-dataObject"
const authorizedIndexKind = "DOAuthorized"
const transactionKind = "TransactionInfo"
const registrationKind = "RegistrationData"

type transactionInfo struct {
	Identifier     string `json:"identifier"`
	Index          string `json:"index"`
	TransactionID  string `json:"transaction_id"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
	UpdatedAtNanos int32  `json:"updated_at_nanos,omitempty"`
}

func rawIndex(stub shim.ChaincodeStubInterface, kind, id string) (string, error) {
	k, e := key(stub, kind, id)
	if e != nil {
		return "", e
	}
	b, e := stub.GetState(k)
	return string(b), e
}

// Identifier mappings store raw values in both directions.
func ensureIndex(stub shim.ChaincodeStubInterface, id string) (string, error) {
	index, e := rawIndex(stub, doiToIndexKind, id)
	if e != nil || index != "" {
		return index, e
	}
	index = base64.StdEncoding.EncodeToString([]byte(stub.GetTxID() + id))
	for _, v := range [][3]string{{doiToIndexKind, id, index}, {indexToDOIKind, index, id}} {
		k, e := key(stub, v[0], v[1])
		if e != nil {
			return "", e
		}
		if e = stub.PutState(k, []byte(v[2])); e != nil {
			return "", e
		}
	}
	return index, nil
}
func link(stub shim.ChaincodeStubInterface, kind, actor, data string, present bool) error {
	a, e := ensureIndex(stub, actor)
	if e != nil {
		return e
	}
	d, e := ensureIndex(stub, data)
	if e != nil {
		return e
	}
	k, e := key(stub, kind, a, d)
	if e != nil {
		return e
	}
	if !present {
		return stub.DelState(k)
	}
	return stub.PutState(k, []byte(d))
}
func touchTransaction(stub shim.ChaincodeStubInterface, id string, created int64) error {
	index, e := ensureIndex(stub, id)
	if e != nil {
		return e
	}
	var info transactionInfo
	if _, e = read(stub, transactionKind, &info, index); e != nil {
		return e
	}
	ts, e := stub.GetTxTimestamp()
	if e != nil {
		return e
	}
	if info.Identifier == "" {
		info = transactionInfo{Identifier: id, Index: index, CreatedAt: created}
	}
	info.TransactionID = stub.GetTxID()
	info.UpdatedAt = ts.Seconds
	info.UpdatedAtNanos = ts.Nanos
	return put(stub, transactionKind, info, index)
}
func persistSourceIndexes(stub shim.ChaincodeStubInterface, s domain.State, r domain.Request, before dataObject, result domain.Result) error {
	switch r.Operation {
	case "identity.register":
		return touchTransaction(stub, result.ObjectID, s.Identities[result.ObjectID].CreatedAt)
	case "data.register", "data.update":
		d := s.Data[result.ObjectID]
		if before.Data.ID != "" && before.Data.OwnerID != d.OwnerID {
			for _, kind := range []string{ownerIndexKind, authorizedIndexKind} {
				if e := link(stub, kind, before.Data.OwnerID, d.ID, false); e != nil {
					return e
				}
			}
		}
		for _, kind := range []string{ownerIndexKind, authorizedIndexKind} {
			if e := link(stub, kind, d.OwnerID, d.ID, true); e != nil {
				return e
			}
		}
		index, e := ensureIndex(stub, d.ID)
		if e != nil {
			return e
		}
		if r.Operation == "data.register" {
			if e = put(stub, registrationKind, d.Profile, index); e != nil {
				return e
			}
		}
		return touchTransaction(stub, d.ID, d.CreatedAt)
	case "authorization.decide", "authorization.update":
		a := s.Authorizations[result.ObjectID]
		if r.Operation == "authorization.update" {
			if e := link(stub, authorizedIndexKind, r.Parameters.Recipient, a.DataID, false); e != nil {
				return e
			}
		}
		// This is an association index, not a proof of currently valid permission.
		if a.Status != domain.AuthorizationPending {
			if e := link(stub, authorizedIndexKind, a.Recipient, a.DataID, true); e != nil {
				return e
			}
		}
		return authorizationTransaction(stub, a, s.Data[a.DataID].CreatedAt)
	}
	return nil
}

func indexedData(stub shim.ChaincodeStubInterface, actor, kind string) ([]domain.DataIdentifier, error) {
	var identity domain.Identity
	exists, err := read(stub, identityKind, &identity, actor)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, domain.ErrNotFound
	}
	actorIndex, err := rawIndex(stub, doiToIndexKind, actor)
	if err != nil {
		return nil, err
	}
	if actorIndex == "" {
		return []domain.DataIdentifier{}, nil
	}
	ts, err := stub.GetTxTimestamp()
	if err != nil {
		return nil, err
	}
	it, err := stub.GetStateByPartialCompositeKey(kind, []string{actorIndex})
	if err != nil {
		return nil, err
	}
	defer it.Close()
	out := []domain.DataIdentifier{}
	seen := map[string]bool{}
	for it.HasNext() {
		entry, err := it.Next()
		if err != nil {
			return nil, err
		}
		id, err := rawIndex(stub, indexToDOIKind, string(entry.Value))
		if err != nil {
			return nil, err
		}
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		d, exists, err := object(stub, id)
		if err != nil {
			return nil, err
		}
		if !exists {
			continue
		}
		allowed := d.Data.OwnerID == actor
		if !allowed && kind == authorizedIndexKind {
			var a domain.AuthorizationRecord
			ok, err := read(stub, authorizationKind, &a, id, actor)
			if err != nil {
				return nil, err
			}
			allowed = ok && a.Status == domain.AuthorizationApproved && a.ExpiresAt > ts.Seconds
		}
		if allowed {
			out = append(out, d.Data)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// indexQuery keeps byte mappings distinct from JSON business records.
func indexQuery(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("%s requires one identifier", function)
	}
	switch function {
	case "TransDOIToIndex", "TransIndexToDOI":
		kind := doiToIndexKind
		if function == "TransIndexToDOI" {
			kind = indexToDOIKind
		}
		v, e := rawIndex(stub, kind, args[0])
		if e != nil {
			return nil, e
		}
		if v == "" {
			return nil, domain.ErrNotFound
		}
		return json.Marshal(v)
	case "GetTransactionInfo":
		index, e := rawIndex(stub, doiToIndexKind, args[0])
		if e != nil {
			return nil, e
		}
		var v transactionInfo
		ok, e := read(stub, transactionKind, &v, index)
		if e != nil {
			return nil, e
		}
		if !ok {
			return nil, domain.ErrNotFound
		}
		v, e = latestAuthorizationTransaction(stub, args[0], v)
		if e != nil {
			return nil, e
		}
		return json.Marshal(v)
	default:
		kind := ownerIndexKind
		if function == "GetAuthorizedData" {
			kind = authorizedIndexKind
		}
		v, e := indexedData(stub, args[0], kind)
		if e != nil {
			return nil, e
		}
		return json.Marshal(v)
	}
}
