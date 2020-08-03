package api

import (
	sq "github.com/Masterminds/squirrel"
	"github.com/RadhiFadlillah/duit/internal/model"
	"database/sql"
	"time"
)

type EntryDao interface{
	Entries(accountId int64, month int, year int) ([]model.Entry,error)
}

type defaultEntryDao struct{
	db   *sql.DB
}

func NewEntryDao(db *sql.DB) EntryDao{
	return &defaultEntryDao{
		db,
	}
}

func (d *defaultEntryDao) Entries(accountId int64, month int, year int) ([]model.Entry,error){
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, -1)

	rows,err := sq.Select(
				"e.id", 
				"e.account_id", 
				"e.affected_account_id",
				"a1.name AS account", 
				"a2.name AS affected_account",
				"e.type", 
				"e.description", 
				"c.name AS category", 
				"e.amount", 
				"e.date").
				From("entry e").
				LeftJoin("account a1 ON e.account_id = a1.id").
				LeftJoin("account a2 ON e.affected_account_id = a2.id").
				LeftJoin("category c ON e.category = c.id").
				Where(sq.Or{
					sq.Eq{"e.account_id": accountId},
					sq.Eq{"e.affected_account_id": accountId},
				}).
				Where(sq.And{
					sq.GtOrEq{
        				"e.date": start,
    				},
    				sq.LtOrEq{
    					"e.date": end,
    				},
    			}).
				OrderBy("e.date DESC, e.id DESC").
				RunWith(d.db).
				Query()

	if err != nil { return nil,err }

	entries := make([]model.Entry, 0)
	for rows.Next() {
        var entry model.Entry
		if err := rows.Scan(
			&entry.ID,
			&entry.AccountID, 
			&entry.AffectedAccountID, 
			&entry.Account,
			&entry.AffectedAccount,
			&entry.Type,
			&entry.Description,
			&entry.Category,
			&entry.Amount,
			&entry.Date,
		); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
    }

    return entries,nil
}