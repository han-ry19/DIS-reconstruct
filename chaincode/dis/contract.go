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
	for _, key := range []string{metadataKey, "dis.metadata.v2", "dis.state.v1"} {
		b, e := stub.GetState(key)
		if e != nil {
			return shim.Error(e.Error())
		}
		if b != nil {
			return shim.Error("already initialized")
		}
	}
	if e := putMetadata(stub, metadata{Domain: args[0]}); e != nil {
		return shim.Error(e.Error())
	}
	return shim.Success(nil)
}
func (c *Contract) Invoke(stub shim.ChaincodeStubInterface) pb.Response {
	function, args := stub.GetFunctionAndParameters()
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
		if s.Sequence != 0 {
			result.Sequence = 0
			result.TransactionID = stub.GetTxID()
			receipt := s.Receipts[request.Actor+"|"+request.RequestID]
			receipt.Result = result
			receipt.Actor = request.Actor
			s.Receipts[request.Actor+"|"+request.RequestID] = receipt
			if e = persist(stub, s, request, d, result); e != nil {
				return shim.Error(e.Error())
			}
			change := domain.Change{Domain: s.Domain, Operation: request.Operation, RequestKey: request.Actor + "|" + request.RequestID}
			switch request.Operation {
			case "identity.register":
				v := s.Identities[result.ObjectID]
				change.Identity = &v
			case "data.register", "data.update":
				v := s.Data[result.ObjectID]
				change.Data = &v
			default:
				v := s.Authorizations[result.ObjectID]
				change.Authorization = &v
				if request.Operation == "authorization.update" && request.Parameters.Field == "recipient" && request.Parameters.NewContent != request.Parameters.Recipient {
					old := v
					old.Recipient = request.Parameters.Recipient
					change.RemovedAuthorizations = []domain.AuthorizationRecord{old}
				}
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
	case "GetOwnedData", "GetAuthorizedData", "TransDOIToIndex", "TransIndexToDOI", "GetTransactionInfo":
		b, err := indexQuery(stub, function, args)
		if err != nil {
			return shim.Error(err.Error())
		}
		return shim.Success(b)
	case "GetDomain":
		if len(args) != 0 {
			return shim.Error("GetDomain takes no arguments")
		}
		return success(m.Domain)
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
		if len(args) != 2 {
			return shim.Error("GetAuthorization requires data and recipient")
		}
		var a domain.AuthorizationRecord
		ok, err := read(stub, authorizationKind, &a, args[0], args[1])
		if err != nil {
			return shim.Error(err.Error())
		}
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
