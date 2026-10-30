package dns

import (
	"github.com/han-ry19/DIS-reconstruct/internal/domain"
	"testing"
)

func TestChangesRejectGapsAndReplay(t *testing.T) {
	s := fixture()
	id := domain.Identity{ID: "next.uid"}
	c := domain.Change{Domain: s.Domain, Sequence: s.Sequence + 1, Operation: "identity.register", Identity: &id}
	if e := ApplyChange(&s, c); e != nil {
		t.Fatal(e)
	}
	if e := ApplyChange(&s, c); e != nil {
		t.Fatal(e)
	}
	c.Sequence += 2
	if e := ApplyChange(&s, c); e == nil {
		t.Fatal("sequence gap accepted")
	}
	c.Domain = "other"
	c.Sequence = s.Sequence + 1
	if e := ApplyChange(&s, c); e == nil {
		t.Fatal("different domain accepted")
	}
}
func TestRedisRecordProjection(t *testing.T) {
	zones, e := encodeRecords(fixture())
	if e != nil {
		t.Fatal(e)
	}
	if zones["uid."]["alice"] == "" || zones["data."]["app.sample"] == "" || zones["data."]["@"] == "" {
		t.Fatal("missing labels", zones)
	}
}
