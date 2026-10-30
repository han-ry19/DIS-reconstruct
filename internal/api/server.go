package api

import (
	"encoding/json"
	"errors"
	"github.com/han-ry19/DIS-reconstruct/internal/backend"
	"github.com/han-ry19/DIS-reconstruct/internal/domain"
	"io"
	"net/http"
)

type Server struct {
	Store backend.Backend
}

func New(store domain.Store) http.Handler {
	return NewBackend(backend.Development{Store: store})
}
func NewBackend(store backend.Backend) http.Handler {
	s := &Server{Store: store}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		write(w, 200, map[string]string{"Status": "ok", "backend": store.Name()})
	})
	mux.HandleFunc("POST /v1/transactions", s.submit)
	mux.HandleFunc("GET /v1/identities/{id}", s.identity)
	mux.HandleFunc("GET /v1/data/{id}", s.data)
	mux.HandleFunc("GET /v1/authorizations", s.authorization)
	return mux
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func problem(w http.ResponseWriter, e error) {
	status := 500
	switch {
	case errors.Is(e, domain.ErrInvalid):
		status = 400
	case errors.Is(e, domain.ErrUnauthorized):
		status = 403
	case errors.Is(e, domain.ErrNotFound):
		status = 404
	case errors.Is(e, domain.ErrConflict):
		status = 409
	}
	message := "internal server error"
	if status != 500 {
		message = e.Error()
	}
	write(w, status, map[string]string{"error": message})
}
func (s *Server) submit(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	var req domain.Request
	if err := d.Decode(&req); err != nil {
		problem(w, domain.ErrInvalid)
		return
	}
	if err := d.Decode(new(any)); err != io.EOF {
		problem(w, domain.ErrInvalid)
		return
	}
	result, err := s.Store.Submit(req)
	if err != nil {
		problem(w, err)
		return
	}
	write(w, 200, result)
}
func (s *Server) identity(w http.ResponseWriter, r *http.Request) {
	if reader, ok := s.Store.(interface {
		GetIdentity(string) (domain.Identity, error)
	}); ok {
		v, e := reader.GetIdentity(r.PathValue("id"))
		if e != nil {
			problem(w, e)
			return
		}
		write(w, 200, v)
		return
	}
	state, e := s.Store.View()
	if e != nil {
		problem(w, e)
		return
	}
	v, ok := state.Identities[r.PathValue("id")]
	if !ok {
		problem(w, domain.ErrNotFound)
		return
	}
	write(w, 200, v)
}
func (s *Server) data(w http.ResponseWriter, r *http.Request) {
	if reader, ok := s.Store.(interface {
		GetData(string) (domain.DataIdentifier, error)
	}); ok {
		v, e := reader.GetData(r.PathValue("id"))
		if e != nil {
			problem(w, e)
			return
		}
		write(w, 200, v)
		return
	}
	state, e := s.Store.View()
	if e != nil {
		problem(w, e)
		return
	}
	v, ok := state.Data[r.PathValue("id")]
	if !ok {
		problem(w, domain.ErrNotFound)
		return
	}
	write(w, 200, v)
}
func (s *Server) authorization(w http.ResponseWriter, r *http.Request) {
	if reader, ok := s.Store.(interface {
		GetAuthorization(string, string, string) (domain.AuthorizationRecord, error)
	}); ok {
		q := r.URL.Query()
		v, e := reader.GetAuthorization(q.Get("data_id"), q.Get("owner_id"), q.Get("recipient"))
		if e != nil {
			problem(w, e)
			return
		}
		write(w, 200, v)
		return
	}
	state, e := s.Store.View()
	if e != nil {
		problem(w, e)
		return
	}
	q := r.URL.Query()
	v, ok := state.Authorizations[domain.AuthorizationKey(q.Get("data_id"), q.Get("owner_id"), q.Get("recipient"))]
	if !ok {
		problem(w, domain.ErrNotFound)
		return
	}
	write(w, 200, v)
}
