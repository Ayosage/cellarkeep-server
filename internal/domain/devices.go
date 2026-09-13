package domain

import (
	"context"

	"github.com/ayosage/cellarkeep-server/internal/auth"
	"github.com/ayosage/cellarkeep-server/internal/store"
)

// Devices is the placeholder device-token lookup. Task 11 replaces it with
// the real hashed-token store. Until then every bearer token is rejected.
type Devices struct{ S *store.Store }

func (d Devices) PrincipalForToken(_ context.Context, _ string) (auth.Principal, error) {
	return auth.Principal{}, ErrForbidden
}
