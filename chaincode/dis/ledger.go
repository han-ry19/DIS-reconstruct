package dis

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/han-ry19/DIS-reconstruct/internal/domain"
	"github.com/hyperledger/fabric-chaincode-go/shim"
)

const legacyKey = "dis.state.v1"
const metadataKey = "dis.metadata.v2"
const identityKind = "Identity"
const dataKind = "DataObject"
const pendingKind = "AuthorizationUnconfirmed"
const receiptKind = "RequestReceipt"

type metadata struct {
	ReceiptSequence bool
	Domain          string
	Sequence        uint64
}
type dataObject struct {
	Data           domain.DataIdentifier                 `json:"data"`
	Authorizations map[string]domain.AuthorizationRecord `json:"authorizations"`
}

func key(stub shim.ChaincodeStubInterface, kind string, attributes ...string) (string, error) {
	k, e := stub.CreateCompositeKey(kind, attributes)
	if e != nil {
		return "", fmt.Errorf("%w: invalid ledger key", domain.ErrInvalid)
	}
	return k, nil
}
func read(stub shim.ChaincodeStubInterface, kind string, target any, attributes ...string) (bool, error) {
	k, e := key(stub, kind, attributes...)
	if e != nil {
		return false, e
	}
	b, e := stub.GetState(k)
	if e != nil || b == nil {
		return false, e
	}
	return true, json.Unmarshal(b, target)
}
func put(stub shim.ChaincodeStubInterface, kind string, value any, attributes ...string) error {
	k, e := key(stub, kind, attributes...)
	if e != nil {
		return e
	}
	b, e := json.Marshal(value)
	if e != nil {
		return e
	}
	return stub.PutState(k, b)
}
func remove(stub shim.ChaincodeStubInterface, kind string, attributes ...string) error {
	k, e := key(stub, kind, attributes...)
	if e != nil {
		return e
	}
	return stub.DelState(k)
}
func getMetadata(stub shim.ChaincodeStubInterface) (metadata, error) {
	var m metadata
	b, e := stub.GetState(metadataKey)
	if e != nil {
		return m, e
	}
	if b == nil {
		return m, fmt.Errorf("ledger requires initialization or MigrateState")
	}
	e = json.Unmarshal(b, &m)
	return m, e
}
func putMetadata(stub shim.ChaincodeStubInterface, m metadata) error {
	b, e := json.Marshal(m)
	if e != nil {
		return e
	}
	return stub.PutState(metadataKey, b)
}
func object(stub shim.ChaincodeStubInterface, id string) (dataObject, bool, error) {
	var d dataObject
	ok, e := read(stub, dataKind, &d, id)
	if d.Authorizations == nil {
		d.Authorizations = map[string]domain.AuthorizationRecord{}
	}
	return d, ok, e
}

// loadRequest reads only the business objects and receipt used by this operation.
func loadRequest(stub shim.ChaincodeStubInterface, m metadata, r domain.Request) (domain.State, dataObject, error) {
	s := domain.NewState(m.Domain)
	s.Sequence = m.Sequence
	var id domain.Identity
	if ok, e := read(stub, identityKind, &id, r.Actor); e != nil {
		return s, dataObject{}, e
	} else if ok {
		s.Identities[id.ID] = id
	}
	var receipt domain.Receipt
	if ok, e := read(stub, receiptKind, &receipt, r.Actor, r.RequestID); e != nil {
		return s, dataObject{}, e
	} else if ok {
		s.Receipts[r.Actor+"|"+r.RequestID] = receipt
	}
	d := dataObject{Authorizations: map[string]domain.AuthorizationRecord{}}
	if r.Parameters.DataID != "" {
		value, ok, e := object(stub, r.Parameters.DataID)
		if e != nil {
			return s, d, e
		}
		d = value
		if ok {
			s.Data[d.Data.ID] = d.Data
			for k, a := range d.Authorizations {
				s.Authorizations[k] = a
			}
			recipient := r.Parameters.Recipient
			if r.Operation == "authorization.request" {
				recipient = r.Actor
			}
			var a domain.AuthorizationRecord
			if found, e := read(stub, pendingKind, &a, d.Data.ID, d.Data.OwnerID, recipient); e != nil {
				return s, d, e
			} else if found {
				s.Authorizations[domain.AuthorizationKey(a.DataID, a.OwnerID, a.Recipient)] = a
			}
		}
	}
	return s, d, nil
}
func scan(stub shim.ChaincodeStubInterface, kind string, fn func([]byte) error) error {
	it, e := stub.GetStateByPartialCompositeKey(kind, nil)
	if e != nil {
		return e
	}
	defer it.Close()
	for it.HasNext() {
		v, e := it.Next()
		if e != nil {
			return e
		}
		if e = fn(v.Value); e != nil {
			return e
		}
	}
	return nil
}
func snapshot(stub shim.ChaincodeStubInterface, m metadata) (domain.State, error) {
	s := domain.NewState(m.Domain)
	s.Sequence = m.Sequence
	e := scan(stub, identityKind, func(b []byte) error {
		var v domain.Identity
		if e := json.Unmarshal(b, &v); e != nil {
			return e
		}
		s.Identities[v.ID] = v
		return nil
	})
	if e != nil {
		return s, e
	}
	e = scan(stub, dataKind, func(b []byte) error {
		var v dataObject
		if e := json.Unmarshal(b, &v); e != nil {
			return e
		}
		s.Data[v.Data.ID] = v.Data
		for k, a := range v.Authorizations {
			s.Authorizations[k] = a
		}
		return nil
	})
	if e != nil {
		return s, e
	}
	e = scan(stub, pendingKind, func(b []byte) error {
		var v domain.AuthorizationRecord
		if e := json.Unmarshal(b, &v); e != nil {
			return e
		}
		s.Authorizations[domain.AuthorizationKey(v.DataID, v.OwnerID, v.Recipient)] = v
		return nil
	})
	if e != nil {
		return s, e
	}
	if m.ReceiptSequence {
		s.ReceiptSequence = true
		s.AppliedRequests = map[string]bool{}
		e = scan(stub, receiptKind, func(b []byte) error {
			var receipt domain.Receipt
			if e := json.Unmarshal(b, &receipt); e != nil {
				return e
			}
			// The receipt stores the actor separately from its public result.
			if receipt.Actor == "" {
				return fmt.Errorf("receipt missing actor")
			}
			s.AppliedRequests[receipt.Actor+"|"+receipt.Result.RequestID] = true
			return nil
		})
		s.Sequence = m.Sequence + uint64(len(s.AppliedRequests))
	}
	return s, e
}
func persist(stub shim.ChaincodeStubInterface, s domain.State, r domain.Request, d dataObject, result domain.Result) error {
	switch r.Operation {
	case "identity.register":
		if e := put(stub, identityKind, s.Identities[result.ObjectID], result.ObjectID); e != nil {
			return e
		}
	case "data.register":
		d.Data = s.Data[result.ObjectID]
		if e := put(stub, dataKind, d, result.ObjectID); e != nil {
			return e
		}
	case "authorization.request":
		a := s.Authorizations[result.ObjectID]
		if e := put(stub, pendingKind, a, a.DataID, a.OwnerID, a.Recipient); e != nil {
			return e
		}
	case "authorization.decide":
		a := s.Authorizations[result.ObjectID]
		d.Authorizations[result.ObjectID] = a
		if e := put(stub, dataKind, d, a.DataID); e != nil {
			return e
		}
		if e := remove(stub, pendingKind, a.DataID, a.OwnerID, a.Recipient); e != nil {
			return e
		}
	}
	if e := put(stub, receiptKind, s.Receipts[r.Actor+"|"+r.RequestID], r.Actor, r.RequestID); e != nil {
		return e
	}
	m, e := getMetadata(stub)
	if e != nil {
		return e
	}
	if m.ReceiptSequence {
		return nil
	}
	return putMetadata(stub, metadata{Domain: s.Domain, Sequence: s.Sequence})
}

// migrate preserves existing identifiers and receipts while replacing the v1 document.
func migrate(stub shim.ChaincodeStubInterface) error {
	existing, e := stub.GetState(metadataKey)
	if e != nil {
		return e
	}
	if existing != nil {
		return nil
	}
	b, e := stub.GetState(legacyKey)
	if e != nil {
		return e
	}
	if b == nil {
		return fmt.Errorf("no legacy state")
	}
	var s domain.State
	if e = json.Unmarshal(b, &s); e != nil {
		return e
	}
	names := make([]string, 0, len(s.Identities))
	for name := range s.Identities {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if e = put(stub, identityKind, s.Identities[name], name); e != nil {
			return e
		}
	}
	names = nil
	for name := range s.Data {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		d := dataObject{Data: s.Data[name], Authorizations: map[string]domain.AuthorizationRecord{}}
		keys := []string{}
		for k, a := range s.Authorizations {
			if a.DataID == name {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		for _, k := range keys {
			a := s.Authorizations[k]
			if a.Status == domain.AuthorizationPending {
				if e = put(stub, pendingKind, a, a.DataID, a.OwnerID, a.Recipient); e != nil {
					return e
				}
			} else {
				d.Authorizations[k] = a
			}
		}
		if e = put(stub, dataKind, d, name); e != nil {
			return e
		}
	}
	names = nil
	for name := range s.Receipts {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		actor, id, ok := strings.Cut(name, "|")
		if !ok {
			return fmt.Errorf("invalid legacy receipt")
		}
		if e = put(stub, receiptKind, s.Receipts[name], actor, id); e != nil {
			return e
		}
	}
	if e = putMetadata(stub, metadata{Domain: s.Domain, Sequence: s.Sequence}); e != nil {
		return e
	}
	if e = stub.DelState(legacyKey); e != nil {
		return e
	}
	return stub.SetEvent(domain.FullUpdateEvent, []byte("{}"))
}
