package api

import (
	"database/sql"
	"fmt"
	sq "github.com/Masterminds/squirrel"
	"github.com/RadhiFadlillah/duit/internal/model"
	"github.com/shopspring/decimal"
)
type AccountDao interface{
	Accounts() ([]*model.Account, error)
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
	).
		From("account").
		RunWith(d.db).
		Query()

	if err != nil {
		return nil, err
	}

	accounts := make([]*model.Account, 0)
	for rows.Next() {
		var account model.Account
		var fInitialAmount float64
		if err := rows.Scan(
			&account.ID,
			&account.Name,
			&fInitialAmount,
		); err != nil {
			return nil, err
		}
		account.InitialAmount,_ = decimal.NewFromString(fmt.Sprintf("%.3f",fInitialAmount))
		accounts = append(accounts, &account)
	}

	return accounts, nil
}