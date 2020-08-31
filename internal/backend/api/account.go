package api

import (
	"encoding/json"
	"net/http"

	"github.com/RadhiFadlillah/duit/internal/model"
	"github.com/julienschmidt/httprouter"
)

// SelectAccounts is handler for GET /api/accounts
func (h *Handler) SelectAccounts(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// Make sure session still valid
	h.auth.MustAuthenticateUser(r)

	accounts, err := h.accountDao.Accounts()
	checkError(err)

	// Return accounts
	w.Header().Add("Content-Encoding", "gzip")
	w.Header().Add("Content-Type", "application/json")
	err = encodeGzippedJSON(w, &accounts)
	checkError(err)
}

// InsertAccount is handler for POST /api/account
func (h *Handler) InsertAccount(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// Make sure session still valid
	h.auth.MustAuthenticateUser(r)

	// Decode request
	var account model.Account
	err := json.NewDecoder(r.Body).Decode(&account)
	checkError(err)

	err = h.accountDao.SaveAccount(&account)
	checkError(err)

	// Return inserted account
	w.Header().Add("Content-Encoding", "gzip")
	w.Header().Add("Content-Type", "application/json")
	err = encodeGzippedJSON(w, &account)
	checkError(err)
}

// UpdateAccount is handler for PUT /api/account
func (h *Handler) UpdateAccount(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// Make sure session still valid
	h.auth.MustAuthenticateUser(r)

	// Decode request
	var account model.Account
	err := json.NewDecoder(r.Body).Decode(&account)
	checkError(err)

	err = h.accountDao.UpdateAccount(&account)
	checkError(err)

	// Return updated account
	w.Header().Add("Content-Encoding", "gzip")
	w.Header().Add("Content-Type", "application/json")
	err = encodeGzippedJSON(w, &account)
	checkError(err)
}

// DeleteAccounts is handler for DELETE /api/accounts
func (h *Handler) DeleteAccounts(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// Make sure session still valid
	h.auth.MustAuthenticateUser(r)

	// Decode request
	var ids []int64
	err := json.NewDecoder(r.Body).Decode(&ids)
	checkError(err)

	_, err = h.accountDao.DeleteAccounts(ids)
	checkError(err)
}
