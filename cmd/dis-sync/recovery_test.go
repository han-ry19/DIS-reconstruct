package main

import (
	"github.com/han-ry19/DIS-reconstruct/internal/domain"
	"testing"
)

func TestRecoverySnapshotContainsTriggeringEvent(t *testing.T) {
	s := domain.NewState("test")
	s.Sequence = 100
	s.ReceiptSequence = true
	s.AppliedRequests = map[string]bool{}
	c := domain.Change{Domain: "test", RequestKey: "alice.uid|update", Sequence: 0}
	if snapshotCoversChange(s, c) {
		t.Fatal("snapshot without triggering receipt accepted")
	}
	s.AppliedRequests[c.RequestKey] = true
	if !snapshotCoversChange(s, c) {
		t.Fatal("snapshot containing triggering receipt rejected")
	}
	c.RequestKey = ""
	c.Sequence = 101
	if snapshotCoversChange(s, c) {
		t.Fatal("legacy snapshot behind event accepted")
	}
	c.Sequence = 100
	if !snapshotCoversChange(s, c) {
		t.Fatal("legacy snapshot at event rejected")
	}
}
