package domain

const ZoneUpdateEvent = "ZoneUpdate"
const FullUpdateEvent = "ZoneFullUpdate"

// Change is the committed business delta consumed by the DNS projection.
type Change struct {
	RequestKey    string               `json:"request_key,omitempty"`
	Domain        string               `json:"domain"`
	Sequence      uint64               `json:"sequence"`
	Operation     string               `json:"operation"`
	Identity      *Identity            `json:"identity,omitempty"`
	Data          *DataIdentifier      `json:"data,omitempty"`
	Authorization *AuthorizationRecord `json:"authorization,omitempty"`
}
