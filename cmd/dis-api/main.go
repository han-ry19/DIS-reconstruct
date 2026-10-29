package main

import (
	"flag"
	"github.com/han-ry19/DIS-reconstruct/internal/api"
	"github.com/han-ry19/DIS-reconstruct/internal/backend"
	"github.com/han-ry19/DIS-reconstruct/internal/fabric"
	"github.com/han-ry19/DIS-reconstruct/internal/store"
	"log"
	"net/http"
	"time"
)

func main() {
	address := flag.String("listen", "127.0.0.1:8080", "HTTP listen address")
	path := flag.String("state", "runtime/state.json", "Development state file")
	network := flag.String("domain", "dis-local", "Request domain")
	fabricConfig := flag.String("fabric-config", "", "Fabric gateway configuration JSON; selects Fabric backend")
	flag.Parse()
	var service backend.Backend
	if *fabricConfig != "" {
		b, e := fabric.Open(*fabricConfig)
		if e != nil {
			log.Fatal(e)
		}
		defer b.Close()
		service = b
	} else {
		st, e := store.Open(*path, *network)
		if e != nil {
			log.Fatal(e)
		}
		defer st.Close()
		service = backend.Development{Store: st}
	}
	server := &http.Server{Addr: *address, Handler: api.NewBackend(service), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 90 * time.Second, IdleTimeout: 60 * time.Second}
	log.Printf("DIS API: %s; backend=%s", *address, service.Name())
	log.Fatal(server.ListenAndServe())
}
