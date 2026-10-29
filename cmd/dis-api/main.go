package main

import (
	"flag"
	"github.com/han-ry19/DIS-reconstruct/internal/api"
	"github.com/han-ry19/DIS-reconstruct/internal/store"
	"log"
	"net/http"
	"time"
)

func main() {
	address := flag.String("listen", "127.0.0.1:8080", "HTTP listen address")
	path := flag.String("state", "runtime/state.json", "Development state file")
	network := flag.String("domain", "dis-local", "Request domain")
	flag.Parse()
	st, err := store.Open(*path, *network)
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()
	server := &http.Server{Addr: *address, Handler: api.New(st), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	log.Printf("DIS API: %s; backend=file-development; domain=%s", *address, *network)
	log.Fatal(server.ListenAndServe())
}
