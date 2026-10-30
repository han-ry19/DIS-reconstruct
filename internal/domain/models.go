package domain

// Identity binds an application or user identifier to an Ed25519 public key.
type Identity struct {
	ID        string `json:"id"`
	PublicKey string `json:"public_key"`
	Kind      string `json:"kind"`
	CreatedAt int64  `json:"created_at"`
}
type DataProfile struct {
	Label       string `json:"label"`
	Format      string `json:"format"`
	Description string `json:"description"`
}
type DataIdentifier struct {
	ID        string      `json:"id"`
	OwnerID   string      `json:"owner_id"`
	Address   string      `json:"address"`
	DataSig   string      `json:"data_signature"`
	Profile   DataProfile `json:"profile"`
	CreatedAt int64       `json:"created_at"`
}
type AuthorizationStatus string

const (
	AuthorizationPending  AuthorizationStatus = "pending"
	AuthorizationApproved AuthorizationStatus = "approved"
	AuthorizationRejected AuthorizationStatus = "rejected"
)

type AuthorizationRecord struct {
	DataID      string              `json:"data_id"`
	OwnerID     string              `json:"owner_id"`
	Recipient   string              `json:"recipient"`
	Status      AuthorizationStatus `json:"status"`
	Permissions []string            `json:"permissions"`
	IssuedAt    int64               `json:"issued_at"`
	ExpiresAt   int64               `json:"expires_at"`
}

// Parameters are typed and serialized in a fixed order for request signing.
type Parameters struct {
	IdentityID    string              `json:"identity_id,omitempty"`
	PublicKey     string              `json:"public_key,omitempty"`
	Kind          string              `json:"kind,omitempty"`
	DataID        string              `json:"data_id,omitempty"`
	Address       string              `json:"address,omitempty"`
	DataSignature string              `json:"data_signature,omitempty"`
	Profile       *DataProfile        `json:"profile,omitempty"`
	Recipient     string              `json:"recipient,omitempty"`
	Permissions   []string            `json:"permissions,omitempty"`
	ExpiresAt     int64               `json:"expires_at,omitempty"`
	Status        AuthorizationStatus `json:"status,omitempty"`
}
type Request struct {
	Domain     string     `json:"domain"`
	Operation  string     `json:"operation"`
	Actor      string     `json:"actor"`
	RequestID  string     `json:"request_id"`
	Timestamp  int64      `json:"timestamp"`
	Parameters Parameters `json:"parameters"`
	Signature  string     `json:"signature"`
}
type Result struct {
	RequestID     string `json:"request_id"`
	Operation     string `json:"operation"`
	ObjectID      string `json:"object_id"`
	Sequence      uint64 `json:"sequence,omitempty"`
	TransactionID string `json:"transaction_id,omitempty"`
	CommittedAt   int64  `json:"committed_at"`
}
type Receipt struct {
	Actor  string `json:"actor,omitempty"`
	Digest string `json:"digest"`
	Result Result `json:"result"`
}
type State struct {
	ReceiptSequence bool                           `json:"receipt_sequence,omitempty"`
	AppliedRequests map[string]bool                `json:"applied_requests,omitempty"`
	Domain          string                         `json:"domain"`
	Sequence        uint64                         `json:"sequence"`
	Identities      map[string]Identity            `json:"identities"`
	Data            map[string]DataIdentifier      `json:"data"`
	Authorizations  map[string]AuthorizationRecord `json:"authorizations"`
	Receipts        map[string]Receipt             `json:"receipts"`
}

func NewState(domain string) State {
	return State{Domain: domain, Identities: map[string]Identity{}, Data: map[string]DataIdentifier{}, Authorizations: map[string]AuthorizationRecord{}, Receipts: map[string]Receipt{}}
}
