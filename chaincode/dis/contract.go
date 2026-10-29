package dis

// Contract operations mirror DIA identity, identifier, and authorization management.
type Contract struct{}

func (Contract) RegisterIdentity() string     { return "identity.register" }
func (Contract) RegisterData() string         { return "data.register" }
func (Contract) RequestAuthorization() string { return "authorization.request" }
func (Contract) DecideAuthorization() string  { return "authorization.decide" }
