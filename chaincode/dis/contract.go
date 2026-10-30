package dis

import (
	"bytes"
	"encoding/json"
	"github.com/han-ry19/DIS-reconstruct/internal/domain"
	"github.com/hyperledger/fabric-chaincode-go/shim"
	pb "github.com/hyperledger/fabric-protos-go/peer"
	"io"
)

type Contract struct{}

func (c *Contract) Init(stub shim.ChaincodeStubInterface) pb.Response {
	_, args := stub.GetFunctionAndParameters()
	if len(args) != 1 || args[0] == "" {
		return shim.Error("Init requires a domain")
	}
	for _, key := range []string{metadataKey, legacyKey} {
		b, e := stub.GetState(key)
		if e != nil {
			return shim.Error(e.Error())
		}
		if b != nil {
			return shim.Error("already initialized")
		}
	}
	if e := putMetadata(stub, metadata{Domain: args[0], ReceiptSequence: true}); e != nil {
		return shim.Error(e.Error())
	}
	return shim.Success(nil)
}
func (c *Contract) Invoke(stub shim.ChaincodeStubInterface) pb.Response {
	function, args := stub.GetFunctionAndParameters()
	if function == "MigrateState" {
		if len(args) != 0 {
			return shim.Error("MigrateState takes no arguments")
		}
		if e := migrate(stub); e != nil {
			return shim.Error(e.Error())
		}
		return shim.Success(nil)
	}
	m, e := getMetadata(stub)
	if e != nil {
		return shim.Error(e.Error())
	}
	switch function {
	case "Submit":
		if len(args) != 1 {
			return shim.Error("Submit requires signed request JSON")
		}
		var request domain.Request
		decoder := json.NewDecoder(bytes.NewBufferString(args[0]))
		decoder.DisallowUnknownFields()
		if e = decoder.Decode(&request); e != nil {
			return shim.Error("invalid request JSON")
		}
		if e = decoder.Decode(new(any)); e != io.EOF {
			return shim.Error("invalid trailing JSON")
		}
		s, d, e := loadRequest(stub, m, request)
		if e != nil {
			return shim.Error(e.Error())
		}
		ts, e := stub.GetTxTimestamp()
		if e != nil {
			return shim.Error(e.Error())
		}
		result, e := domain.Apply(&s, request, ts.Seconds)
		if e != nil {
			return shim.Error(e.Error())
		}
		if s.Sequence != m.Sequence {
			if m.ReceiptSequence {
				result.Sequence = 0
				result.TransactionID = stub.GetTxID()
				receipt := s.Receipts[request.Actor+"|"+request.RequestID]
				receipt.Result = result
				receipt.Actor = request.Actor
				s.Receipts[request.Actor+"|"+request.RequestID] = receipt
			}
			if e = persist(stub, s, request, d, result); e != nil {
				return shim.Error(e.Error())
			}
			change := domain.Change{Domain: s.Domain, Sequence: s.Sequence, Operation: request.Operation}
			if m.ReceiptSequence {
				change.Sequence = 0
				change.RequestKey = request.Actor + "|" + request.RequestID
			}
			switch request.Operation {
			case "identity.register":
				v := s.Identities[result.ObjectID]
				change.Identity = &v
			case "data.register":
				v := s.Data[result.ObjectID]
				change.Data = &v
			default:
				v := s.Authorizations[result.ObjectID]
				change.Authorization = &v
			}
			b, e := json.Marshal(change)
			if e != nil {
				return shim.Error(e.Error())
			}
			if e = stub.SetEvent(domain.ZoneUpdateEvent, b); e != nil {
				return shim.Error(e.Error())
			}
		}
		return success(result)
	case "GetSnapshot":
		if len(args) != 0 {
			return shim.Error("GetSnapshot takes no arguments")
		}
		s, e := snapshot(stub, m)
		if e != nil {
			return shim.Error(e.Error())
		}
		return success(s)
	case "GetIdentity":
		if len(args) != 1 {
			return shim.Error("GetIdentity requires an identifier")
		}
		var v domain.Identity
		ok, e := read(stub, identityKind, &v, args[0])
		if e != nil {
			return shim.Error(e.Error())
		}
		if !ok {
			return shim.Error("not found")
		}
		return success(v)
	case "GetData":
		if len(args) != 1 {
			return shim.Error("GetData requires an identifier")
		}
		v, ok, e := object(stub, args[0])
		if e != nil {
			return shim.Error(e.Error())
		}
		if !ok {
			return shim.Error("not found")
		}
		return success(v.Data)
	case "GetAuthorization":
		if len(args) != 3 {
			return shim.Error("GetAuthorization requires data, owner, recipient")
		}
		var a domain.AuthorizationRecord
		ok, e := read(stub, pendingKind, &a, args...)
		if e != nil {
			return shim.Error(e.Error())
		}
		if ok {
			return success(a)
		}
		d, _, e := object(stub, args[0])
		if e != nil {
			return shim.Error(e.Error())
		}
		a, ok = d.Authorizations[domain.AuthorizationKey(args[0], args[1], args[2])]
		if !ok {
			return shim.Error("not found")
		}
		return success(a)
	default:
		return shim.Error("unknown function")
	}
}
func success(value any) pb.Response {
	b, e := json.Marshal(value)
	if e != nil {
		return shim.Error(e.Error())
	}
	return shim.Success(b)
}
