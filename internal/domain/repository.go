package domain

// Store atomically publishes a state transition and its request receipt.
// A failed callback must leave committed state unchanged.
type Store interface {
	Update(func(*State) (Result, error)) (Result, error)
	View() (State, error)
}
