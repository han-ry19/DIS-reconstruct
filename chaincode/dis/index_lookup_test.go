package dis

import (
	"encoding/json"
	"testing"

	"github.com/han-ry19/DIS-reconstruct/internal/domain"
)

func TestIndexQueriesDoNotScanUnrelatedObjectsOrReceipts(t *testing.T) {
	h := newManagementHarness(t)
	for _, kind := range []string{receiptKind, dataKind} {
		k, err := key(h.stub, kind, "unrelated")
		if err != nil {
			t.Fatal(err)
		}
		h.stub.State[k] = []byte("invalid JSON outside this query")
	}
	for _, method := range []string{"GetOwnedData", "GetAuthorizedData"} {
		var result []domain.DataIdentifier
		if err := json.Unmarshal(h.query(method, "alice.uid"), &result); err != nil {
			t.Fatal(err)
		}
		if len(result) != 1 || result[0].ID != "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data" {
			t.Fatalf("%s: %+v", method, result)
		}
	}
}
