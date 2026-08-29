package out

import "context"

// Transactor abstracts database transaction coordination.
// Implementations store the transaction handle in the context so repository
// methods can retrieve it transparently via their context parameter.
type Transactor interface {
	InTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}
