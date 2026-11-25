package disredis

import (
	"context"
	mdns "github.com/miekg/dns"
	"net"
	"testing"
)

type remoteTransferWriter struct {
	transferWriter
	remote net.Addr
}

func (w *remoteTransferWriter) RemoteAddr() net.Addr { return w.remote }

func TestZoneTransferAccess(t *testing.T) {
	h := Handler{Zones: []string{"uid.", "data."}}
	request := new(mdns.Msg)
	request.SetAxfr("data.")
	for _, address := range []net.Addr{&net.TCPAddr{IP: net.ParseIP("192.0.2.10")}, &net.UDPAddr{IP: net.ParseIP("127.0.0.1")}} {
		writer := &remoteTransferWriter{remote: address}
		code, err := h.ServeDNS(context.Background(), writer, request)
		if err != nil || code != mdns.RcodeRefused {
			t.Fatalf("unexpected transfer response: %d %v", code, err)
		}
	}
	if !h.transferAllowed(&net.TCPAddr{IP: net.ParseIP("127.0.0.1")}) || !h.transferAllowed(&net.TCPAddr{IP: net.ParseIP("::1")}) {
		t.Fatal("default loopback transfer denied")
	}
	_, network, _ := net.ParseCIDR("192.0.2.0/24")
	h.TransferNetworks = []*net.IPNet{network}
	if !h.transferAllowed(&net.TCPAddr{IP: net.ParseIP("192.0.2.10")}) || h.transferAllowed(&net.TCPAddr{IP: net.ParseIP("198.51.100.10")}) {
		t.Fatal("transfer allowlist ignored")
	}
	h.TransferNetworks = []*net.IPNet{}
	if h.transferAllowed(&net.TCPAddr{IP: net.ParseIP("127.0.0.1")}) {
		t.Fatal("explicit deny-all ignored")
	}
}
