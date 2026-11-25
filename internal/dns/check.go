package dns

import (
	"encoding/json"
	"fmt"
	"github.com/gomodule/redigo/redis"
)

// LoadCheckpoint reads only publication metadata; it does not scan cached objects.
func LoadCheckpoint(address string) (ProjectionCheckpoint, bool, error) {
	var p ProjectionCheckpoint
	c, err := ConnectRedis(address)
	if err != nil {
		return p, false, err
	}
	defer c.Close()
	raw, err := redis.Bytes(c.Do("GET", cacheKey))
	if err == redis.ErrNil {
		return p, false, nil
	}
	if err != nil {
		return p, false, err
	}
	err = json.Unmarshal(raw, &p)
	return p, err == nil, err
}

// CheckProjection checks the serving projection, independently of ledger history.
// It does not establish that the consumer has caught up to the channel head.
func CheckProjection(address, authoritative string, nsAddress ...string) (ProjectionCheckpoint, error) {
	p, ok, err := LoadProjection(address)
	if err != nil {
		return p, err
	}
	if !ok || p.State.Domain == "" {
		return p, fmt.Errorf("missing Redis projection checkpoint")
	}
	if authoritative == "" {
		return p, nil
	}
	if err = Verify(p.State, authoritative, nsAddress...); err != nil {
		return p, err
	}
	current, ok, err := LoadCheckpoint(address)
	if err != nil {
		return p, err
	}
	if !ok || current.State.Domain != p.State.Domain || current.State.Sequence != p.State.Sequence || current.Block != p.Block || current.Transaction != p.Transaction {
		return p, fmt.Errorf("projection advanced during verification; retry")
	}
	return p, nil
}
