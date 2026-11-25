package fabric

import (
	"context"
	"encoding/json"
	"github.com/han-ry19/DIS-reconstruct/internal/domain"
	"github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/hyperledger/fabric-protos-go-apiv2/peer"
	"google.golang.org/protobuf/proto"
	"testing"
)

func replayedView(b *Backend) (domain.State, error) {
	s, _, err := b.Rebuild(context.Background())
	return s, err
}
func TestRecoverySkipsInvalidAndForeignTransactions(t *testing.T) {
	marshal := func(m proto.Message) []byte {
		b, e := proto.Marshal(m)
		if e != nil {
			t.Fatal(e)
		}
		return b
	}
	makeTx := func(chaincode string) []byte {
		payload, _ := json.Marshal(domain.Change{Domain: "test", Operation: "identity.register", Identity: &domain.Identity{ID: "alice.uid"}})
		event := marshal(&peer.ChaincodeEvent{ChaincodeId: chaincode, EventName: domain.ZoneUpdateEvent, Payload: payload})
		ca := marshal(&peer.ChaincodeAction{Events: event})
		prp := marshal(&peer.ProposalResponsePayload{Extension: ca})
		cap := marshal(&peer.ChaincodeActionPayload{Action: &peer.ChaincodeEndorsedAction{ProposalResponsePayload: prp}})
		tx := marshal(&peer.Transaction{Actions: []*peer.TransactionAction{{Payload: cap}}})
		header := marshal(&common.ChannelHeader{Type: int32(common.HeaderType_ENDORSER_TRANSACTION)})
		body := marshal(&common.Payload{Header: &common.Header{ChannelHeader: header}, Data: tx})
		return marshal(&common.Envelope{Payload: body})
	}
	metadata := make([][]byte, int(common.BlockMetadataIndex_TRANSACTIONS_FILTER)+1)
	metadata[int(common.BlockMetadataIndex_TRANSACTIONS_FILTER)] = []byte{byte(peer.TxValidationCode_VALID), byte(peer.TxValidationCode_MVCC_READ_CONFLICT), byte(peer.TxValidationCode_VALID)}
	block := &common.Block{Data: &common.BlockData{Data: [][]byte{makeTx("dis"), makeTx("dis"), makeTx("other")}}, Metadata: &common.BlockMetadata{Metadata: metadata}}
	changes, err := committedChanges(block, "dis")
	if err != nil || len(changes) != 1 || changes[0].Identity.ID != "alice.uid" {
		t.Fatal(changes, err)
	}
	block.Metadata.Metadata[int(common.BlockMetadataIndex_TRANSACTIONS_FILTER)] = []byte{0}
	if _, err = committedChanges(block, "dis"); err == nil {
		t.Fatal("incomplete validation flags accepted")
	}
}
