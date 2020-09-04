package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"github.com/RadhiFadlillah/duit/internal/backend/utils"
	"github.com/RadhiFadlillah/duit/internal/model"
	"github.com/julienschmidt/httprouter"
	"sort"
	"reflect"
)

// SelectUsers is handler for GET /api/users
func (h *Handler) SelectUsers(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// Make sure session still valid
	h.auth.MustAuthenticateUser(r)

	// Fetch from database
	users,err := h.userDao.Users()
	checkError(err)

	// Return list of users
	w.Header().Add("Content-Encoding", "gzip")
	w.Header().Add("Content-Type", "application/json")
	err = encodeGzippedJSON(w, &users)
	checkError(err)
}

// InsertUser is handler for POST /api/user
func (h *Handler) InsertUser(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// Decode request
	var user model.User
	err := json.NewDecoder(r.Body).Decode(&user)
	checkError(err)

	// Validate input
	if user.Name == "" {
		panic(fmt.Errorf("name must not empty"))
	}

	if user.Username == "" {
		panic(fmt.Errorf("username must not empty"))
	}

	if user.Password == "" {
		user.Password = utils.RandomString(10)
	}

	// If admin already exists, make sure session still valid
	adminIds, err := h.userDao.AdminIds()
	checkError(err)

	if len(adminIds) > 0 {
		h.auth.MustAuthenticateUser(r)
	}

	err = h.userDao.SaveUser(&user)
	checkError(err)

	// Return inserted user
	w.Header().Add("Content-Encoding", "gzip")
	w.Header().Add("Content-Type", "application/json")
	err = encodeGzippedJSON(w, &user)
	checkError(err)
}

// DeleteUsers is handler for DELETE /api/users
func (h *Handler) DeleteUsers(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// Make sure session still valid
	h.auth.MustAuthenticateUser(r)

	// Decode request
	var ids []int64
	err := json.NewDecoder(r.Body).Decode(&ids)
	checkError(err)

	adminIds, err := h.userDao.AdminIds()
	sort.Slice(adminIds, func (i,j int) bool { return adminIds[i] < adminIds[j] })
	sort.Slice(ids, func(i,j int) bool { return ids[i] < ids[j]})

	if reflect.DeepEqual(adminIds, ids){
		panic(fmt.Errorf("There must be atleast one admin account"))
	}

	usernames, err := h.userDao.DeleteUsers(ids)
	checkError(err)
	
	// Delete from database
	for _, username := range usernames {
		h.auth.MassLogout(username)
	}
}

// UpdateUser is handler for PUT /api/user
func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// Make sure session still valid
	h.auth.MustAuthenticateUser(r)

	// Decode request
	var user model.User
	err := json.NewDecoder(r.Body).Decode(&user)
	checkError(err)

	// Validate input
	if user.Name == "" {
		panic(fmt.Errorf("name must not empty"))
	}

	if user.Username == "" {
		panic(fmt.Errorf("username must not empty"))
	}

	// Start transaction
	// Make sure to rollback if panic ever happened
	adminIds, err := h.userDao.AdminIds()

	if len(adminIds) == 1 && 
		adminIds[0] == user.ID &&
		user.Admin == false {
		panic(fmt.Errorf("Assign another account as admin before revoking admin privilege on this account"))
	}

	oldUser, err := h.userDao.FindUserById(user.ID)
	checkError(err)

	err = h.userDao.UpdateUser(&user)
	checkError(err)

	// If username or admin status changed, do mass logout
	if oldUser.Username != user.Username || oldUser.Admin != user.Admin {
		h.auth.MassLogout(oldUser.Username)
	}

	// Return updated user
	w.Header().Add("Content-Encoding", "gzip")
	w.Header().Add("Content-Type", "application/json")
	err = encodeGzippedJSON(w, &user)
	checkError(err)
}

// ChangeUserPassword is handler for PUT /api/user/password
func (h *Handler) ChangeUserPassword(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// Decode request
	var request struct {
		UserID      int64    `json:"userId"`
		OldPassword string `json:"oldPassword"`
		NewPassword string `json:"newPassword"`
	}

	err := json.NewDecoder(r.Body).Decode(&request)
	checkError(err)

	username,err := h.userDao.ChangePassword(request.UserID, request.OldPassword, request.NewPassword)
	checkError(err)

	// Do mass logout for this account
	h.auth.MassLogout(username)
}

// ResetUserPassword is handler for PUT /api/user/password/reset
func (h *Handler) ResetUserPassword(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// Make sure session still valid
	h.auth.MustAuthenticateUser(r)

	// Decode request
	var id int64
	err := json.NewDecoder(r.Body).Decode(&id)
	checkError(err)

	credentials,err := h.userDao.ResetPassword(id)

	// Do mass logout for this user
	h.auth.MassLogout(credentials.Username())

	// Return new passwords
	result := struct {
		ID       int64    `json:"id"`
		Password string `json:"password"`
	}{id, credentials.Password()}

	w.Header().Add("Content-Encoding", "gzip")
	w.Header().Add("Content-Type", "application/json")
	err = encodeGzippedJSON(w, &result)
	checkError(err)
}
