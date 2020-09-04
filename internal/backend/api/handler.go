package api

import (
	"github.com/RadhiFadlillah/duit/internal/backend/auth"
	"github.com/RadhiFadlillah/duit/internal/backend/repo"
	"github.com/jmoiron/sqlx"
)

// Handler represents handler for every API routes.
type Handler struct {
	db         *sqlx.DB
	auth       *auth.Authenticator
	entryDao   repo.EntryDao
	accountDao repo.AccountDao
	userDao	   repo.UserDao
}

// NewHandler returns new Handler
func NewHandler(db *sqlx.DB, auth *auth.Authenticator) (*Handler, error) {
	// Create handler
	handler := new(Handler)
	handler.db = db
	handler.auth = auth
	handler.entryDao = repo.NewEntryDao(db.DB)
	handler.accountDao = repo.NewAccountDao(db.DB)
	handler.userDao = repo.NewUserDao(db.DB)
	return handler, nil
}
