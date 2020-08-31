package api

import (
	"github.com/RadhiFadlillah/duit/internal/backend/auth"
	"github.com/jmoiron/sqlx"
)

// Handler represents handler for every API routes.
type Handler struct {
	db         *sqlx.DB
	auth       *auth.Authenticator
	entryDao   EntryDao
	accountDao AccountDao
}

// NewHandler returns new Handler
func NewHandler(db *sqlx.DB, auth *auth.Authenticator) (*Handler, error) {
	// Create handler
	handler := new(Handler)
	handler.db = db
	handler.auth = auth
	handler.entryDao = NewEntryDao(db.DB)
	handler.accountDao = NewAccountDao(db.DB)
	return handler, nil
}
