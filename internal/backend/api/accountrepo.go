package api

import (
	"database/sql"
	"fmt"
	sq "github.com/Masterminds/squirrel"
	"github.com/RadhiFadlillah/duit/internal/model"
	"github.com/shopspring/decimal"
)

type AccountDao interface {
	Accounts() ([]*model.Account, error)
	SaveAccount(account *model.Account) error
	FindAccountById(accountId int64) (*model.Account, error)
	UpdateAccount(entry *model.Account) error
	DeleteAccounts(ids []int64) (int64, error)
}

type defaultAccountDao struct {
	db *sql.DB
}

func NewAccountDao(db *sql.DB) AccountDao {
	return &defaultAccountDao{
		db,
	}
}

func (d *defaultAccountDao) Accounts() ([]*model.Account, error) {
	rows, err := sq.Select(
		"id",
		"name",
		"initial_amount",
		"total",
	).
		From("account_total").
		OrderBy("name").
		RunWith(d.db).
		Query()

	if err != nil {
		return nil, err
	}

	accounts := make([]*model.Account, 0)
	for rows.Next() {
		var account model.Account
		var fInitialAmount float64
		var fTotalAmount float64
		if err := rows.Scan(
			&account.ID,
			&account.Name,
			&fInitialAmount,
			&fTotalAmount,
		); err != nil {
			return nil, err
		}
		account.InitialAmount, _ = decimal.NewFromString(fmt.Sprintf("%.3f", fInitialAmount))
		account.Total, _ = decimal.NewFromString(fmt.Sprintf("%.3f", fTotalAmount))
		accounts = append(accounts, &account)
	}

	return accounts, nil
}

func (d *defaultAccountDao) FindAccountById(accountId int64) (*model.Account, error) {

	rows, err := sq.Select(
		"id",
		"name",
		"initial_amount",
		"total",
	).
		From("account_total").
		Where(sq.And{
			sq.Eq{"id": accountId},
		}).
		RunWith(d.db).
		Query()

	if err != nil {
		return nil, err
	}

	var account model.Account
	var fInitialAmount float64
	var fTotalAmount float64
	if rows.Next() {
		if err := rows.Scan(
			&account.ID,
			&account.Name,
			&fInitialAmount,
			&fTotalAmount,
		); err != nil {
			return nil, err
		}
	}
	account.InitialAmount, _ = decimal.NewFromString(fmt.Sprintf("%.3f", fInitialAmount))
	account.Total, _ = decimal.NewFromString(fmt.Sprintf("%.3f", fTotalAmount))

	return &account, nil
}

func (d *defaultAccountDao) SaveAccount(account *model.Account) error {

	//Begin Transaction
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}

	// Save Entries

	res, err := sq.
		Insert("account").
		Columns(
			"name",
			"initial_amount",
		).Values(
		account.Name,
		account.InitialAmount,
	).
		RunWith(tx).
		Exec()

	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return rollbackErr
		}
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

	account.ID = lastInsertedID
	return nil
}

func (d *defaultAccountDao) UpdateAccount(account *model.Account) error {
	if account == nil || account.ID == 0 {
		return fmt.Errorf("Can't update nil account")
	}

	//Begin Transaction
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}

	res, err := sq.
		Update("account").
		Set("name", account.Name).
		Set("initial_amount", account.InitialAmount).
		Where(sq.And{
			sq.Eq{"id": account.ID},
		}).
		RunWith(tx).
		Exec()

	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return rollbackErr
		}
		return err
	}
	// Commit
	if err := tx.Commit(); err != nil {
		return err
	}

	if rowsAffected, _ := res.RowsAffected(); rowsAffected == 0 {
		return fmt.Errorf("Account not found: %q", account.ID)
	}

	updatedAccount, err := d.FindAccountById(account.ID)
	if err != nil {
		return err
	}

	account = updatedAccount
	return nil
}

func (d *defaultAccountDao) DeleteAccounts(ids []int64) (int64, error) {
	tx, err := d.db.Begin()
	if err != nil {
		return 0, err
	}

	res, err := sq.
		Delete("account").
		Where(sq.And{
			sq.Eq{"id": ids},
		}).
		RunWith(tx).
		Exec()

	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return 0, rollbackErr
		}
		return 0, err
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return 0, err
	}

	rowsAffected, _ := res.RowsAffected()
	return rowsAffected, nil
}
