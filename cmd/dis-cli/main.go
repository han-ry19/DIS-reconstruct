package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/han-ry19/DIS-reconstruct/internal/domain"
	"os"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: dis-cli keygen | sign")
	}
	switch os.Args[1] {
	case "keygen":
		fs := flag.NewFlagSet("keygen", flag.ContinueOnError)
		out := fs.String("out", "identity.key", "New private-key file")
		if e := fs.Parse(os.Args[2:]); e != nil {
			return e
		}
		pub, key, e := ed25519.GenerateKey(rand.Reader)
		if e != nil {
			return e
		}
		f, e := os.OpenFile(*out, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			return e
		}
		_, e = f.WriteString(base64.StdEncoding.EncodeToString(key) + "\n")
		ce := f.Close()
		if e != nil {
			return e
		}
		if ce != nil {
			return ce
		}
		fmt.Println(base64.StdEncoding.EncodeToString(pub))
		return nil
	case "sign":
		fs := flag.NewFlagSet("sign", flag.ContinueOnError)
		keyPath := fs.String("key", "identity.key", "Private-key file")
		input := fs.String("request", "request.json", "Unsigned request JSON")
		if e := fs.Parse(os.Args[2:]); e != nil {
			return e
		}
		keyData, e := os.ReadFile(*keyPath)
		if e != nil {
			return e
		}
		var keyText string
		_, e = fmt.Sscan(string(keyData), &keyText)
		if e != nil {
			return e
		}
		key, e := base64.StdEncoding.DecodeString(keyText)
		if e != nil {
			return e
		}
		b, e := os.ReadFile(*input)
		if e != nil {
			return e
		}
		var req domain.Request
		if e = json.Unmarshal(b, &req); e != nil {
			return e
		}
		if e = domain.Sign(&req, ed25519.PrivateKey(key)); e != nil {
			return e
		}
		return json.NewEncoder(os.Stdout).Encode(req)
	default:
		return fmt.Errorf("unknown command %q", os.Args[1])
	}
}
