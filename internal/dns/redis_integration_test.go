package dns

import (
	"encoding/json"
	"github.com/gomodule/redigo/redis"
	"github.com/han-ry19/DIS-reconstruct/internal/domain"
	"os"
	"testing"
)

// Use an isolated Redis instance: this test owns the DIS keys in that instance.
func TestLiveRedisProjectionRecovery(t *testing.T) {
	address := os.Getenv("DIS_TEST_REDIS_ADDRESS")
	if address == "" {
		t.Skip("set DIS_TEST_REDIS_ADDRESS to an isolated Redis instance")
	}
	c, e := ConnectRedis(address)
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	keys := []any{cacheKey, "uid.", "data.", "_dis:identities", "_dis:data", "_dis:authorizations"}
	if _, e = c.Do("DEL", keys...); e != nil {
		t.Fatal(e)
	}
	s := fixture()
	p := ProjectionCheckpoint{State: s, Block: 10, Transaction: "first"}
	if e = SaveProjection(address, p, true); e != nil {
		t.Fatal(e)
	}
	recovered, ok, e := LoadProjection(address)
	if e != nil || !ok {
		t.Fatal(e)
	}
	a, _ := Digest(s)
	b, _ := Digest(recovered.State)
	if a != b || recovered.Block != 10 {
		t.Fatal("full recovery changed state")
	}
	id := domain.Identity{ID: "extra.uid", Kind: "user", PublicKey: s.Identities["alice.uid"].PublicKey}
	change := domain.Change{Domain: s.Domain, Sequence: s.Sequence + 1, Operation: "identity.register", Identity: &id}
	if e = ApplyChange(&p.State, change); e != nil {
		t.Fatal(e)
	}
	p.Change = &change
	p.Block = 11
	p.Transaction = "second"
	if e = SaveProjection(address, p, false); e != nil {
		t.Fatal(e)
	}
	recovered, ok, e = LoadProjection(address)
	if e != nil || !ok || recovered.State.Identities[id.ID].ID != id.ID || recovered.Transaction != "second" {
		t.Fatal(recovered, e)
	}
	raw, _ := redis.Bytes(c.Do("GET", cacheKey))
	var metadata ProjectionCheckpoint
	json.Unmarshal(raw, &metadata)
	if metadata.State.Identities != nil {
		t.Fatal("checkpoint contains full business state")
	}
	if _, e = redis.String(c.Do("HGET", "uid.", "extra")); e != nil {
		t.Fatal("incremental DNS record missing", e)
	}
	if _, e = redis.String(c.Do("HGET", "uid.", "alice")); e != nil {
		t.Fatal("unrelated DNS record removed", e)
	}
	if _, e = redis.String(c.Do("HGET", "data.", "sample")); e != nil {
		t.Fatal("unrelated zone removed", e)
	}
	stale := recovered
	stale.State.Sequence--
	if e = SaveProjection(address, stale, true); e == nil {
		t.Fatal("stale publication accepted")
	}
	p.Change = nil
	p.Transaction = "replayed"
	if e = SaveProjection(address, p, false); e != nil {
		t.Fatal(e)
	}
}
