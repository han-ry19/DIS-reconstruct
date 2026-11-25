package fabric

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/han-ry19/DIS-reconstruct/internal/dns"
	"github.com/han-ry19/DIS-reconstruct/internal/domain"
	"github.com/hyperledger/fabric-gateway/pkg/client"
	"github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/hyperledger/fabric-protos-go-apiv2/peer"
	"google.golang.org/protobuf/proto"
)

// Rebuild replays valid committed events to a captured block boundary. It keeps
// current business objects only; receipts remain in Fabric for request retries.
// The returned height is the first block not included in the recovered state.
func (b *Backend) Rebuild(ctx context.Context) (domain.State, uint64, error) {
	raw, err := b.contract.EvaluateTransaction("GetDomain")
	if err != nil {
		return domain.State{}, 0, err
	}
	var name string
	if err = json.Unmarshal(raw, &name); err != nil {
		return domain.State{}, 0, err
	}
	s := domain.NewState(name)
	raw, err = b.network.GetContract("qscc").EvaluateTransaction("GetChainInfo", b.network.Name())
	if err != nil {
		return s, 0, err
	}
	var info common.BlockchainInfo
	if err = proto.Unmarshal(raw, &info); err != nil {
		return s, 0, err
	}
	if info.Height == 0 {
		return s, 0, fmt.Errorf("empty channel")
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	blocks, err := b.network.BlockEvents(ctx, client.WithStartBlock(0))
	if err != nil {
		return s, 0, err
	}
	var expected uint64
	for {
		select {
		case <-ctx.Done():
			return s, 0, ctx.Err()
		case block, ok := <-blocks:
			if !ok {
				return s, 0, fmt.Errorf("recovery block stream closed")
			}
			if block.GetHeader().GetNumber() != expected {
				return s, 0, fmt.Errorf("recovery block gap")
			}
			events, err := committedChanges(block, b.chaincode)
			if err != nil {
				return s, 0, err
			}
			for _, change := range events {
				change.Sequence = s.Sequence + 1
				if err = dns.ApplyChange(&s, change); err != nil {
					return s, 0, err
				}
			}
			expected++
			if expected == info.Height {
				return s, expected, nil
			}
		}
	}
}

func committedChanges(block *common.Block, chaincode string) ([]domain.Change, error) {
	var out []domain.Change
	index := int(common.BlockMetadataIndex_TRANSACTIONS_FILTER)
	if block.Metadata == nil || len(block.Metadata.Metadata) <= index {
		return nil, fmt.Errorf("missing block validation flags")
	}
	flags := block.Metadata.Metadata[index]
	if len(flags) != len(block.GetData().GetData()) {
		return nil, fmt.Errorf("invalid block validation flags")
	}
	for i, raw := range block.GetData().GetData() {
		if peer.TxValidationCode(flags[i]) != peer.TxValidationCode_VALID {
			continue
		}
		var env common.Envelope
		var payload common.Payload
		var header common.ChannelHeader
		if err := proto.Unmarshal(raw, &env); err != nil {
			return nil, err
		}
		if err := proto.Unmarshal(env.Payload, &payload); err != nil {
			return nil, err
		}
		if err := proto.Unmarshal(payload.GetHeader().GetChannelHeader(), &header); err != nil {
			return nil, err
		}
		if common.HeaderType(header.Type) != common.HeaderType_ENDORSER_TRANSACTION {
			continue
		}
		var tx peer.Transaction
		if err := proto.Unmarshal(payload.Data, &tx); err != nil {
			return nil, err
		}
		for _, action := range tx.Actions {
			var cap peer.ChaincodeActionPayload
			var prp peer.ProposalResponsePayload
			var ca peer.ChaincodeAction
			var event peer.ChaincodeEvent
			if err := proto.Unmarshal(action.Payload, &cap); err != nil {
				return nil, err
			}
			if cap.Action == nil {
				return nil, fmt.Errorf("missing endorsed action")
			}
			if err := proto.Unmarshal(cap.Action.ProposalResponsePayload, &prp); err != nil {
				return nil, err
			}
			if err := proto.Unmarshal(prp.Extension, &ca); err != nil {
				return nil, err
			}
			if len(ca.Events) == 0 {
				continue
			}
			if err := proto.Unmarshal(ca.Events, &event); err != nil {
				return nil, err
			}
			if event.ChaincodeId != chaincode {
				continue
			}
			if event.EventName != domain.ZoneUpdateEvent {
				return nil, fmt.Errorf("unsupported DIS event %s", event.EventName)
			}
			var change domain.Change
			if err := json.Unmarshal(event.Payload, &change); err != nil {
				return nil, err
			}
			out = append(out, change)
		}
	}
	return out, nil
}
