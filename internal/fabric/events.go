package fabric

import (
	"context"
	"github.com/hyperledger/fabric-gateway/pkg/client"
)

func (b *Backend) Events(ctx context.Context, checkpoint client.Checkpoint) (<-chan *client.ChaincodeEvent, error) {
	return b.network.ChaincodeEvents(ctx, b.chaincode, client.WithStartBlock(0), client.WithCheckpoint(checkpoint))
}
