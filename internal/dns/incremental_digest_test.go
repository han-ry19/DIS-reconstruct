package dns

import (
	"encoding/json"
	"github.com/gomodule/redigo/redis"
	"github.com/han-ry19/DIS-reconstruct/internal/domain"
	"os"
	"testing"
)

func TestLiveIncrementalCheckpointAndSameSequenceConflict(t *testing.T) {
	address := os.Getenv("DIS_TEST_REDIS_ADDRESS")
	if address == "" {
		t.Skip("set DIS_TEST_REDIS_ADDRESS to an isolated Redis instance")
	}
	conn, err := ConnectRedis(address)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err = conn.Do("DEL", cacheKey, "uid.", "data.", "_dis:identities", "_dis:data", "_dis:authorizations", "_dis:applied"); err != nil {
		t.Fatal(err)
	}
	s := fixture()
	s.ReceiptSequence = true
	s.AppliedRequests = map[string]bool{}
	p := ProjectionCheckpoint{State: s, Block: 1, Transaction: "initial"}
	if err = SaveProjection(address, p, true); err != nil {
		t.Fatal(err)
	}
	id := domain.Identity{ID: "new.uid", Kind: "user", PublicKey: s.Identities["alice.uid"].PublicKey}
	c := domain.Change{Domain: s.Domain, Operation: "identity.register", RequestKey: "new.uid|register", Identity: &id}
	if err = ApplyChange(&p.State, c); err != nil {
		t.Fatal(err)
	}
	p.Change = &c
	p.Block = 2
	p.Transaction = "next"
	if err = SaveProjection(address, p, false); err != nil {
		t.Fatal(err)
	}
	raw, err := redis.Bytes(conn.Do("GET", cacheKey))
	if err != nil {
		t.Fatal(err)
	}
	var meta ProjectionCheckpoint
	if err = json.Unmarshal(raw, &meta); err != nil {
		t.Fatal(err)
	}
	if meta.Digest != "" {
		t.Fatal("incremental path computed a full state digest")
	}
	recovered, ok, err := LoadProjection(address)
	if err != nil || !ok {
		t.Fatal(err)
	}
	altered := recovered.State.Data["sample.data"]
	altered.Address = "https://example.org/different"
	recovered.State.Data[altered.ID] = altered
	if err = SaveProjection(address, recovered, true); err == nil {
		t.Fatal("same-sequence conflicting snapshot accepted")
	}
	current, ok, err := LoadProjection(address)
	if err != nil || !ok {
		t.Fatal(err)
	}
	if current.State.Data[altered.ID].Address == altered.Address {
		t.Fatal("rejected snapshot changed cache")
	}
	if err = SaveProjection(address, current, true); err != nil {
		t.Fatal("matching full snapshot rejected", err)
	}
}
