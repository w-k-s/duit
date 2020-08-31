package api

import (
	"github.com/julienschmidt/httprouter"
	"net/http"
)

// SelectCategories is handler for GET /api/categories
func (h *Handler) SelectCategories(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// Make sure session still valid
	h.auth.MustAuthenticateUser(r)

	// Get URL parameter
	accountID := strToInt(r.URL.Query().Get("account"))

	categories, err := h.entryDao.Categories(int64(accountID))
	checkError(err)

	// Return list of categories
	w.Header().Add("Content-Encoding", "gzip")
	w.Header().Add("Content-Type", "application/json")
	err = encodeGzippedJSON(w, &categories)
	checkError(err)
}
