package dns

import (
	"github.com/han-ry19/DIS-reconstruct/internal/domain"
	"testing"
)

func TestReceiptEventsDeduplicateSnapshotAndCountIndependentChanges(t *testing.T) {
	s := domain.NewState("test")
	s.ReceiptSequence = true
	s.Sequence = 1
	s.AppliedRequests = map[string]bool{"owner.uid|already-snapshotted": true}
	current := domain.Identity{ID: "owner.uid", PublicKey: "current"}
	s.Identities[current.ID] = current
	old := domain.Identity{ID: "owner.uid", PublicKey: "old"}
	replay := domain.Change{Domain: "test", RequestKey: "owner.uid|already-snapshotted", Operation: "identity.register", Identity: &old}
	if err := ApplyChange(&s, replay); err != nil {
		t.Fatal(err)
	}
	if s.Sequence != 1 || s.Identities[current.ID].PublicKey != "current" {
		t.Fatal("snapshot replay changed current state")
	}
	next := domain.Identity{ID: "next.uid"}
	change := domain.Change{Domain: "test", RequestKey: "next.uid|register", Operation: "identity.register", Identity: &next}
	if err := ApplyChange(&s, change); err != nil {
		t.Fatal(err)
	}
	if err := ApplyChange(&s, change); err != nil {
		t.Fatal(err)
	}
	if s.Sequence != 2 || len(s.AppliedRequests) != 2 || s.Identities[next.ID].ID != next.ID {
		t.Fatal("independent event count or deduplication failed")
	}
}
