package backend

import (
	"github.com/han-ry19/DIS-reconstruct/internal/domain"
	"time"
)

type Backend interface {
	Submit(domain.Request) (domain.Result, error)
	View() (domain.State, error)
	Name() string
}
type Development struct{ Store domain.Store }

func (d Development) Submit(r domain.Request) (domain.Result, error) {
	return d.Store.Update(func(s *domain.State) (domain.Result, error) { return domain.Apply(s, r, time.Now().Unix()) })
}
func (d Development) View() (domain.State, error) { return d.Store.View() }
func (d Development) Name() string                { return "file-development" }
