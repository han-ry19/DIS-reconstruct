package store

import (
	"encoding/json"
	"errors"
	"github.com/han-ry19/DIS-reconstruct/internal/domain"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

// File is a single-process development store. Fabric provides distributed commits.
type File struct {
	mu    sync.Mutex
	path  string
	state domain.State
	lock  *os.File
}

func Open(path, network string) (*File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		lock.Close()
		return nil, err
	}
	f := &File{path: path, state: domain.NewState(network), lock: lock}
	b, err := os.ReadFile(path)
	if err == nil {
		err = json.Unmarshal(b, &f.state)
	} else if errors.Is(err, os.ErrNotExist) {
		err = nil
	}
	if err == nil && (f.state.Domain != network || f.state.Identities == nil || f.state.Data == nil || f.state.Authorizations == nil || f.state.Receipts == nil) {
		err = errors.New("invalid state or domain mismatch")
	}
	if err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}
func (f *File) Close() error { return f.lock.Close() }
func clone(s domain.State) (domain.State, error) {
	b, e := json.Marshal(s)
	if e != nil {
		return domain.State{}, e
	}
	var out domain.State
	e = json.Unmarshal(b, &out)
	return out, e
}
func (f *File) View() (domain.State, error) { f.mu.Lock(); defer f.mu.Unlock(); return clone(f.state) }
func (f *File) Update(fn func(*domain.State) (domain.Result, error)) (domain.Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	next, err := clone(f.state)
	if err != nil {
		return domain.Result{}, err
	}
	result, err := fn(&next)
	if err != nil {
		return domain.Result{}, err
	}
	if next.Sequence == f.state.Sequence {
		return result, nil
	}
	b, err := json.Marshal(next)
	if err != nil {
		return domain.Result{}, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(f.path), ".dis-state-*")
	if err != nil {
		return domain.Result{}, err
	}
	defer os.Remove(tmp.Name())
	if _, err = tmp.Write(b); err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return domain.Result{}, err
	}
	if err = os.Rename(tmp.Name(), f.path); err != nil {
		return domain.Result{}, err
	}
	// A rename makes the new state visible even if a subsequent directory sync fails.
	f.state = next
	dir, err := os.Open(filepath.Dir(f.path))
	if err != nil {
		return domain.Result{}, err
	}
	defer dir.Close()
	if err = dir.Sync(); err != nil {
		return domain.Result{}, err
	}
	return result, nil
}
