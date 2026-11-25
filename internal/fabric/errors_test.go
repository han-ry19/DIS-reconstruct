package fabric

import (
	"errors"
	"fmt"
	"github.com/han-ry19/DIS-reconstruct/internal/domain"
	"github.com/hyperledger/fabric-gateway/pkg/client"
	peer "github.com/hyperledger/fabric-protos-go-apiv2/peer"
	"testing"
)

func TestCommitConflictClassification(t *testing.T) {
	for _, code := range []peer.TxValidationCode{peer.TxValidationCode_MVCC_READ_CONFLICT, peer.TxValidationCode_PHANTOM_READ_CONFLICT} {
		original := &client.CommitError{TransactionID: "test-transaction", Code: code}
		if err := classify(fmt.Errorf("submit: %w", original)); !errors.Is(err, domain.ErrConflict) {
			t.Fatalf("%v: %v", code, err)
		}
	}
	original := &client.CommitError{TransactionID: "test-transaction", Code: peer.TxValidationCode_ENDORSEMENT_POLICY_FAILURE}
	if errors.Is(classify(original), domain.ErrConflict) {
		t.Fatal("policy failure classified as retryable conflict")
	}
}
