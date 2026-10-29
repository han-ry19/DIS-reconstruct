package domain

import (
	"context"
	"time"
)

type Repository interface {
	RegisterIdentity(context.Context, Identity) error
	GetIdentity(context.Context, string) (Identity, error)
	RegisterData(context.Context, DataIdentifier) error
	GetData(context.Context, string) (DataIdentifier, error)
	CreateAuthorization(context.Context, AuthorizationRecord) error
	DecideAuthorization(context.Context, string, string, AuthorizationStatus, time.Time) error
	GetAuthorization(context.Context, string, string) (AuthorizationRecord, error)
}
