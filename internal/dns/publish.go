package dns

import (
	"encoding/json"
	"fmt"
	"github.com/han-ry19/DIS-reconstruct/internal/domain"
	mdns "github.com/miekg/dns"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

type Status struct {
	Domain    string `json:"domain"`
	Sequence  uint64 `json:"sequence"`
	Digest    string `json:"digest"`
	WrittenAt string `json:"written_at"`
}

func atomic(path string, b []byte) error {
	f, e := os.CreateTemp(filepath.Dir(path), ".publish-*")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if _, e = f.Write(b); e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	if e = os.Rename(f.Name(), path); e != nil {
		return e
	}
	dir, e := os.Open(filepath.Dir(path))
	if e != nil {
		return e
	}
	defer dir.Close()
	return dir.Sync()
}
func Publish(s domain.State, directory string) error {
	zones, e := Project(s)
	if e != nil {
		return e
	}
	digest, e := Digest(s)
	if e != nil {
		return e
	}
	if e = os.MkdirAll(directory, 0700); e != nil {
		return e
	}
	lock, e := os.OpenFile(filepath.Join(directory, "publisher.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return e
	}
	defer lock.Close()
	if e = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); e != nil {
		return e
	}
	path := filepath.Join(directory, "status.json")
	previous, e := os.ReadFile(path)
	if e == nil {
		var status Status
		if e = json.Unmarshal(previous, &status); e != nil {
			return e
		}
		if status.Domain != s.Domain || status.Sequence > s.Sequence || (status.Sequence == s.Sequence && status.Digest != digest) {
			return fmt.Errorf("publication would replace a different or newer state")
		}
	} else if !os.IsNotExist(e) {
		return e
	}
	for _, name := range []string{"uid.", "data."} {
		var body strings.Builder
		for _, rr := range zones[name] {
			body.WriteString(rr.String() + "\n")
		}
		if e = atomic(filepath.Join(directory, strings.TrimSuffix(name, ".")+".zone"), []byte(body.String())); e != nil {
			return e
		}
	}
	b, e := json.MarshalIndent(Status{Domain: s.Domain, Sequence: s.Sequence, Digest: digest, WrittenAt: time.Now().UTC().Format(time.RFC3339Nano)}, "", "  ")
	if e != nil {
		return e
	}
	return atomic(path, b)
}

// Verify checks the complete authoritative zone contents by TCP AXFR.
func Verify(s domain.State, address string) error {
	expected, e := Project(s)
	if e != nil {
		return e
	}
	for _, zone := range []string{"uid.", "data."} {
		message := new(mdns.Msg)
		message.SetAxfr(zone)
		transfer := &mdns.Transfer{DialTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 5 * time.Second}
		envelopes, e := transfer.In(message, address)
		if e != nil {
			return e
		}
		actual := []string{}
		soa := 0
		for envelope := range envelopes {
			if envelope.Error != nil {
				return envelope.Error
			}
			for _, rr := range envelope.RR {
				if rr.Header().Rrtype == mdns.TypeSOA {
					soa++
					if soa > 1 {
						continue
					}
				}
				actual = append(actual, rr.String())
			}
		}
		wanted := []string{}
		for _, rr := range expected[zone] {
			wanted = append(wanted, rr.String())
		}
		sort.Strings(actual)
		sort.Strings(wanted)
		if strings.Join(actual, "\n") != strings.Join(wanted, "\n") {
			return fmt.Errorf("zone %s does not match sequence %d", zone, s.Sequence)
		}
	}
	return nil
}
