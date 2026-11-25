package dis

import (
	"encoding/json"
	"github.com/han-ry19/DIS-reconstruct/internal/domain"
	"testing"
	"time"
)

func TestIndexAuthorizationUsesCurrentBusinessState(t *testing.T) {
	h := newManagementHarness(t)
	query := func() int {
		var result []domain.DataIdentifier
		if err := json.Unmarshal(h.query("GetAuthorizedData", "app.uid"), &result); err != nil {
			t.Fatal(err)
		}
		return len(result)
	}
	p := domain.Parameters{DataID: "item.data", Recipient: "app.uid", Permissions: []string{"Read"}, ExpiresAt: time.Now().Add(time.Hour).Unix()}
	h.submit("authorization.request", "app.uid", p, true)
	if query() != 0 {
		t.Fatal("pending request granted access")
	}
	h.submit("authorization.decide", "alice.uid", domain.Parameters{DataID: p.DataID, Recipient: p.Recipient, Status: domain.AuthorizationApproved}, true)
	if query() != 1 {
		t.Fatal("approved association not returned")
	}
	h.submit("authorization.update", "alice.uid", domain.Parameters{DataID: p.DataID, Recipient: p.Recipient, Field: "status", NewContent: "rejected"}, true)
	if query() != 0 {
		t.Fatal("stale association granted rejected access")
	}
	h.submit("authorization.decide", "alice.uid", domain.Parameters{DataID: p.DataID, Recipient: p.Recipient, Status: domain.AuthorizationApproved}, true)
	h.submit("authorization.update", "alice.uid", domain.Parameters{DataID: p.DataID, Recipient: p.Recipient, Field: "expires_at", NewContent: "1"}, true)
	if query() != 0 {
		t.Fatal("expired association returned")
	}
	h.submit("authorization.request", "app.uid", p, true)
	if query() != 0 {
		t.Fatal("pending overlay exposed prior authorization")
	}
}
