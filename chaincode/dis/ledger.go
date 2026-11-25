package dis

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/han-ry19/DIS-reconstruct/internal/domain"
	"github.com/hyperledger/fabric-chaincode-go/shim"
)

const metadataKey = "dis.metadata.v4"
const identityKind = "Identity"
const dataKind = "DataObject"
const receiptKind = "RequestReceipt"

type metadata struct{ Domain string }
type dataObject struct {
	Data domain.DataIdentifier `json:"data"`
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
		return m, fmt.Errorf("ledger requires initialization for schema 4")
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
	return d, ok, e
}

// loadRequest reads only the business objects and receipt used by this operation.
func loadRequest(stub shim.ChaincodeStubInterface, m metadata, r domain.Request) (domain.State, dataObject, error) {
	s := domain.NewState(m.Domain)
	s.Sequence = 0
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
	d := dataObject{}
	if r.Parameters.DataID != "" {
		value, ok, e := object(stub, r.Parameters.DataID)
		if e != nil {
			return s, d, e
		}
		d = value
		if ok {
			s.Data[d.Data.ID] = d.Data
			if r.Operation == "data.update" && r.Parameters.Field == "RP" || r.Operation == "authorization.update" && r.Parameters.Field == "recipient" {
				var next domain.Identity
				if found, err := read(stub, identityKind, &next, r.Parameters.NewContent); err != nil {
					return s, d, err
				} else if found {
					s.Identities[next.ID] = next
				}
			}
			if err := loadAuthorization(stub, &s, d.Data.ID, r.Parameters.Recipient); err != nil {
				return s, d, err
			}
			if r.Operation == "authorization.update" && r.Parameters.Field == "recipient" {
				if err := loadAuthorization(stub, &s, d.Data.ID, r.Parameters.NewContent); err != nil {
					return s, d, err
				}
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
	s.Sequence = 0
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
		return nil
	})
	if e != nil {
		return s, e
	}
	e = scan(stub, authorizationKind, func(b []byte) error {
		var a domain.AuthorizationRecord
		if err := json.Unmarshal(b, &a); err != nil {
			return err
		}
		s.Authorizations[domain.AuthorizationKey(a.DataID, a.OwnerID, a.Recipient)] = a
		return nil
	})
	return s, e
}

func persist(stub shim.ChaincodeStubInterface, s domain.State, r domain.Request, d dataObject, result domain.Result) error {
	before := d
	if strings.HasPrefix(r.Operation, "authorization.") {
		if err := persistIndependentAuthorization(stub, s, r, result); err != nil {
			return err
		}
	} else if r.Operation == "identity.register" {
		if err := put(stub, identityKind, s.Identities[result.ObjectID], result.ObjectID); err != nil {
			return err
		}
	} else {
		d.Data = s.Data[result.ObjectID]
		if err := put(stub, dataKind, d, result.ObjectID); err != nil {
			return err
		}
	}
	if e := persistSourceIndexes(stub, s, r, before, result); e != nil {
		return e
	}
	if e := put(stub, receiptKind, s.Receipts[r.Actor+"|"+r.RequestID], r.Actor, r.RequestID); e != nil {
		return e
	}
	return nil
}
