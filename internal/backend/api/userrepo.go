package api

import (
	"database/sql"
	"fmt"
	sq "github.com/Masterminds/squirrel"
	"github.com/RadhiFadlillah/duit/internal/model"
	"golang.org/x/crypto/bcrypt"
)

type UserDao interface {
	Users() ([]*model.User, error)
	FindUserById(userId int64) (*model.User, error)
	SaveUser(user *model.User) error
	// Batch deletes users with ids, returns usernames of all deleted users
	// Query will not be executed if there wouldn't be any admin users left after the transaction
	DeleteUsers(ids []int64)([]string,error)
	usernamesForIds(ids []int64) (map[int64]string, error)
	// Updates user details; ignores password
	// Query will not be executed If user is the only admin and update would revoke user's admin status
	UpdateUser(user *model.User) error
	// Hashes and saves user's new password if old password is correct 
	// Returns username that is updated
	ChangePassword(userId int64, oldPassword string, newPassword string) (string,error)
	// Hashes and saves user's new password without checking old password
	// Returns username that is updated
	ResetPassword(userId int64) (model.Credentials, error)
	// Ids of admin users
	AdminIds()([]int64,error)
}

type defaultUserDao struct {
	db *sql.DB
}

func NewUserDao(db *sql.DB) UserDao {
	return &defaultUserDao{
		db,
	}
}

func (d *defaultUserDao) Users() ([]*model.User, error) {
	rows, err := sq.Select(
		"id",
		"username",
		"name",
		"admin",
	).
		From("user").
		OrderBy("name").
		RunWith(d.db).
		Query()

	if err != nil {
		return nil, err
	}

	users := make([]*model.User, 0)
	for rows.Next() {
		var user model.User
		if err := rows.Scan(
			&user.ID,
			&user.Username,
			&user.Name,
			&user.Admin,
		); err != nil {
			return nil, err
		}
		users = append(users, &user)
	}

	return users, nil
}

func (d *defaultUserDao) AdminIds()([]int64,error){
	rows, err := sq.Select(
		"id",
	).
		From("user").
		Where(sq.And{
			sq.Eq{"admin": 1},
		}).
		RunWith(d.db).
		Query()

	if err != nil{
		return []int64{}, err
	}

	adminIds := []int64{}
	for rows.Next() {
		var adminId int64
		if err := rows.Scan(&adminId); err != nil {
			// Make sure to get all admin ids or return error
			return []int64{}, err
		}
		adminIds = append(adminIds,adminId)
	}
	
	return adminIds, nil
}

func (d *defaultUserDao) SaveUser(user *model.User) error {
	// Hash password with bcrypt
	password := []byte(user.Password)
	hashedPassword, err := bcrypt.GenerateFromPassword(password, 10)
	checkError(err)

	//Begin Transaction
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Save Users
	res, err := sq.
		Insert("user").
		Columns(
			"username",
			"name",
			"password",
			"admin",
		).Values(
			user.Username,
			user.Name,
			hashedPassword,
			user.Admin,
	).
		RunWith(tx).
		Exec()

	if err != nil {
		return err
	}

	// Commit
	if err := tx.Commit(); err != nil {
		return err
	}

	lastInsertedID, err := res.LastInsertId()
	if err != nil {
		return err
	}

	user.ID = lastInsertedID
	return nil
}

func (d *defaultUserDao) FindUserById(userId int64) (*model.User, error) {

	rows, err := sq.Select(
		"id",
		"username",
		"password",
		"name",
		"admin",
	).
		From("user").
		Where(sq.Eq{"id": userId}).
		RunWith(d.db).
		Query()

	if err != nil || !rows.Next(){
		return nil, err
	}

	var user model.User
	if err := rows.Scan(
		&user.ID,
		&user.Username,
		&user.Password,
		&user.Name,
		&user.Admin,
	); err != nil {
		return nil, err
	}

	return &user, nil
}

// Updates user details; ignores password
// Query will not be executed If user is the only admin and update would revoke user's admin status
func (d *defaultUserDao) UpdateUser(user *model.User) error {
	if user == nil || user.ID == 0 {
		return fmt.Errorf("Can't update nil account")
	}

	//Begin Transaction
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := sq.
		Update("user").
		Set("username", user.Username).
		Set("name", user.Name).
		Set("admin", user.Admin).
		Where(sq.Eq{"id": user.ID}).
		RunWith(tx).
		Exec()

	if err != nil {
		return err
	}
	// Commit
	if err := tx.Commit(); err != nil {
		return err
	}

	if rowsAffected, _ := res.RowsAffected(); rowsAffected == 0 {
		return fmt.Errorf("User not found: %q", user.ID)
	}

	updatedUser, err := d.FindUserById(user.ID)
	if err != nil {
		return err
	}

	user = updatedUser
	return nil
}

// Batch deletes users with ids, returns usernames of all deleted users
// Query will not be executed if there wouldn't be any admin users left after the transaction
func (d *defaultUserDao) DeleteUsers(ids []int64)([]string,error) {
	tx, err := d.db.Begin()
	if err != nil {
		return []string{}, err
	}
	defer tx.Rollback()

	idAndUsernames,err := d.usernamesForIds(ids)
	if err != nil{
		return []string{},err
	}
	usernames := []string{}
	for _,username := range idAndUsernames{
		usernames = append(usernames,username)
	}

	res, err := sq.
		Delete("user").
		Where(sq.And{
			sq.Eq{"id": ids},
		}).
		RunWith(tx).
		Exec()

	if err != nil {
		return []string{}, err
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected != int64(len(ids)) {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return []string{}, fmt.Errorf("Rollback failed when Aborting deletion because not all ids in %q could be deleted", ids)
		}
		return []string{}, fmt.Errorf("Aborted deletion because not all ids in %q could be deleted", ids)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return []string{}, err
	}

	return usernames, nil
}

func (d *defaultUserDao) usernamesForIds(ids []int64) (map[int64]string, error) {

	rows, err := sq.Select(
		"id",
		"username",
	).
		From("user").
		Where(sq.Eq{"id": ids}).
		RunWith(d.db).
		Query()

	if err != nil {
		return nil, err
	}

	usernames := map[int64]string{}
	for rows.Next() {
		var id int64
		var username string
		if err := rows.Scan(
			&id,
			&username,
		); err != nil {
			continue
		}
		usernames[id] = username
	}

	return usernames, nil
}

// Hashes and saves user's new password if old password is correct 
// Returns username that is updated
func (d *defaultUserDao) ChangePassword(userId int64, oldPassword string, newPassword string) (string,error){
	if userId == 0 {
		return "",fmt.Errorf("Invalid user id")
	}

	//Begin Transaction
	tx, err := d.db.Begin()
	if err != nil {
		return "",err
	}
	defer tx.Rollback()

	// Get username
	user,err := d.FindUserById(userId)
	if err != nil{
		return "",err
	}

	// Compare old password with database
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(oldPassword))
	if err != nil {
		return "",fmt.Errorf("old password for %s doesn't match", user.Username)
	}

	// Hash the new password with bcrypt
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), 10)
	if err != nil{
		return "",err
	}

	res, err := sq.
		Update("user").
		Set("password", hashedPassword).
		Where(sq.Eq{"id": user.ID}).
		RunWith(tx).
		Exec()

	if err != nil {
		return "",err
	}

	// Commit
	if err := tx.Commit(); err != nil {
		return "",err
	}

	if rowsAffected, _ := res.RowsAffected(); rowsAffected == 0 {
		return "",fmt.Errorf("User not found: %q", user.ID)
	}

	return user.Username,nil
}

// Hashes and saves user's new password without checking old password
// Returns username that is updated
func (d *defaultUserDao) ResetPassword(userId int64) (model.Credentials, error){
	if userId == 0 {
		return model.Credentials{},fmt.Errorf("Invalid user id")
	}

	//Begin Transaction
	tx, err := d.db.Begin()
	if err != nil {
		return model.Credentials{},err
	}
	defer tx.Rollback()

	// Get username
	user,err := d.FindUserById(userId)
	if err != nil{
		return model.Credentials{},err
	}

	// Generate password and hash with bcrypt
	password := []byte(randomString(10))
	hashedPassword, err := bcrypt.GenerateFromPassword(password, 10)
	if err != nil{
		return model.Credentials{},err
	}

	res, err := sq.
		Update("user").
		Set("password", hashedPassword).
		Where(sq.Eq{"id": user.ID}).
		RunWith(tx).
		Exec()

	if err != nil {
		return model.Credentials{},err
	}

	// Commit
	if err := tx.Commit(); err != nil {
		return model.Credentials{},err
	}

	if rowsAffected, _ := res.RowsAffected(); rowsAffected == 0 {
		return model.Credentials{},fmt.Errorf("User not found: %q", user.ID)
	}

	return model.CreateCredentials(user.Username, string(password)),nil
}