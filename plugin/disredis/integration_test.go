package disredis

import (
	"context"
	"fmt"
	"github.com/gomodule/redigo/redis"
	projection "github.com/han-ry19/DIS-reconstruct/internal/dns"
	"github.com/han-ry19/DIS-reconstruct/internal/domain"
	mdns "github.com/miekg/dns"
	"net"
	"os"
	"testing"
)

type transferWriter struct{ messages []*mdns.Msg }

func (w *transferWriter) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 53}
}
func (w *transferWriter) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 40000}
}
func (w *transferWriter) WriteMsg(m *mdns.Msg) error {
	if _, e := m.Pack(); e != nil {
		return e
	}
	w.messages = append(w.messages, m.Copy())
	return nil
}
func (w *transferWriter) Write(b []byte) (int, error) { return len(b), nil }
func (w *transferWriter) Close() error                { return nil }
func (w *transferWriter) TsigStatus() error           { return nil }
func (w *transferWriter) TsigTimersOnly(bool)         {}
func (w *transferWriter) Hijack()                     {}
func TestLiveRedisLargeZoneTransfer(t *testing.T) {
	address := os.Getenv("DIS_TEST_REDIS_ADDRESS")
	if address == "" {
		t.Skip("set DIS_TEST_REDIS_ADDRESS to an isolated Redis instance")
	}
	state := domain.NewState("large-test")
	state.Sequence = 1
	for i := 0; i < 600; i++ {
		id := fmt.Sprintf("object-%d.data", i)
		state.Data[id] = domain.DataIdentifier{ID: id, OwnerID: "owner.uid", Address: "https://example.org/objects/long-path-to-original-data", DataSig: "stored-signature", Profile: domain.DataProfile{Description: "large zone transfer acceptance"}}
	}
	c, e := projection.ConnectRedis(address)
	if e != nil {
		t.Fatal(e)
	}
	keys := []any{"_dis:projection:v1", "uid.", "data.", "_dis:identities", "_dis:data", "_dis:authorizations"}
	if _, e = c.Do("DEL", keys...); e != nil {
		t.Fatal(e)
	}
	c.Close()
	if e = projection.SaveProjection(address, projection.ProjectionCheckpoint{State: state}, true); e != nil {
		t.Fatal(e)
	}
	h := Handler{Zones: []string{"uid.", "data."}, Pool: &redis.Pool{Dial: func() (redis.Conn, error) { return projection.ConnectRedis(address) }}}
	defer h.Pool.Close()
	request := new(mdns.Msg)
	request.SetAxfr("data.")
	writer := &transferWriter{}
	code, e := h.ServeDNS(context.Background(), writer, request)
	if e != nil || code != mdns.RcodeSuccess {
		t.Fatal(code, e)
	}
	expected, _ := projection.Project(state)
	actual := 0
	for _, message := range writer.messages {
		actual += len(message.Answer)
	}
	if len(writer.messages) < 2 || actual != len(expected["data."])+1 {
		t.Fatalf("incomplete multi-message transfer: messages=%d records=%d expected=%d", len(writer.messages), actual, len(expected["data."])+1)
	}
}
