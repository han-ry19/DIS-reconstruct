package api

import (
	"encoding/json"
	"errors"
	"github.com/han-ry19/DIS-reconstruct/internal/domain"
	"net/http/httptest"
	"testing"
)

func TestConflictProblemCodes(t *testing.T) {
	for _, tc := range []struct {
		err   error
		code  string
		retry bool
	}{
		{domain.ErrConflict, "conflict", false},
		{errors.Join(domain.ErrConflict, domain.ErrLedgerConflict), "ledger_conflict", true},
	} {
		w := httptest.NewRecorder()
		problem(w, tc.err)
		var v struct {
			Code      string
			Retryable bool
		}
		if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
			t.Fatal(err)
		}
		if w.Code != 409 || v.Code != tc.code || v.Retryable != tc.retry {
			t.Fatal(w.Code, w.Body.String())
		}
		if (w.Header().Get("Retry-After") != "") != tc.retry {
			t.Fatal("incorrect retry header")
		}
	}
}
