package domain

import "time"

type Identity struct {
	ID        string
	PublicKey string
	Kind      string
	CreatedAt time.Time
}
type DataProfile struct {
	Label       string
	Format      string
	Description string
}
type DataIdentifier struct {
	ID        string
	OwnerID   string
	Address   string
	DataSig   string
	Profile   DataProfile
	CreatedAt time.Time
}
type AuthorizationStatus string

const (
	AuthorizationPending  AuthorizationStatus = "pending"
	AuthorizationApproved AuthorizationStatus = "approved"
	AuthorizationRejected AuthorizationStatus = "rejected"
)

type AuthorizationRecord struct {
	DataID      string
	OwnerID     string
	Recipient   string
	Status      AuthorizationStatus
	Permissions []string
	IssuedAt    time.Time
	ExpiresAt   time.Time
}
