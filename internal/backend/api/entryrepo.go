package api

import (
	"database/sql"
	"fmt"
	sq "github.com/Masterminds/squirrel"
	"github.com/RadhiFadlillah/duit/internal/model"
	"github.com/shopspring/decimal"
	"time"
)

type TimeRange struct {
	StartDate time.Time
	EndDate   time.Time
}

func ForMonth(month int, year int) *TimeRange {
	startDate := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	return &TimeRange{
		startDate,
		startDate.AddDate(0, 1, -1),
	}
}

type EntryDao interface {
	Entries(accountId int64, tr *TimeRange) ([]*model.Entry, error)
	SaveEntries(entries []*model.Entry) error
	SaveEntry(entry *model.Entry) error
	UpdateEntry(entry *model.Entry) error
	FindCategoriesByName(name []string, accountId int64) ([]*model.Category, error)
	CreateCategoriesIfNotExist([]*model.Category) ([]*model.Category, error)
	CreateCategoryIfNotExists(category *model.Category) (*model.Category, error)
	Categories(accountId int64) ([]*model.Category, error)
	DeleteEntries(ids []int64) (int64, error)
	GetMininumAndMaximumExpenseForYear(year int) (*model.ExpenseRange, error)
	GetMonthStartBalanceForYear(year int) ([]*model.ChartSeries, error)
	GetTotalExpensePerCategoryForMonth(accountId int64, month int, categoryType model.Type) ([]*model.CategoryExpensesSummary, error)
}

type defaultEntryDao struct {
	db *sql.DB
}

func NewEntryDao(db *sql.DB) EntryDao {
	return &defaultEntryDao{
		db,
	}
}

func (d *defaultEntryDao) Entries(accountId int64, tr *TimeRange) ([]*model.Entry, error) {
	rows, err := sq.Select(
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
				"e.date": tr.StartDate,
			},
			sq.LtOrEq{
				"e.date": tr.EndDate,
			},
		}).
		OrderBy("e.date DESC, e.id DESC").
		RunWith(d.db).
		Query()

	if err != nil {
		return nil, err
	}

	entries := make([]*model.Entry, 0)
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
		entries = append(entries, &entry)
	}

	return entries, nil
}

func (d *defaultEntryDao) SaveEntries(entries []*model.Entry) error {
	if len(entries) == 0 {
		return nil
	}

	accountID := entries[0].AccountID

	// Collect category names for all entries
	entryCategories := make([]*model.Category, 0, len(entries))
	for _, entry := range entries {
		entryCategories = append(entryCategories, &model.Category{
			AccountID: accountID,
			Name:      entry.Category.ValueOrZero(),
			Type:      entry.Type,
		})
	}

	//Begin Transaction
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}

	categories, err := d.CreateCategoriesIfNotExist(entryCategories)
	if err != nil {
		return err
	}

	categoriesIDs := make(map[string]int64)
	for _, category := range categories {
		categoriesIDs[category.Name] = category.ID
	}

	// Save Entries

	ib := sq.
		Insert("entry").
		Columns(
			"account_id",
			"affected_account_id",
			"type",
			"description",
			"amount",
			"date",
			"category",
		)

	for _, entry := range entries {
		ib = ib.Values(
			entry.AccountID,
			entry.AffectedAccountID,
			entry.Type,
			entry.Description,
			entry.Amount,
			entry.Date,
			categoriesIDs[entry.Category.ValueOrZero()],
		)
	}

	_, err = ib.RunWith(d.db).Exec()
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

	return nil
}

func (d *defaultEntryDao) SaveEntry(entry *model.Entry) error {

	accountID := entry.AccountID

	//Begin Transaction
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}

	category, err := d.CreateCategoryIfNotExists(&model.Category{
		AccountID: accountID,
		Name:      entry.Category.ValueOrZero(),
		Type:      entry.Type,
	})
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return rollbackErr
		}
		return err
	}

	// Save Entries

	res, err := sq.
		Insert("entry").
		Columns(
			"account_id",
			"affected_account_id",
			"type",
			"description",
			"amount",
			"date",
			"category",
		).Values(
		entry.AccountID,
		entry.AffectedAccountID,
		entry.Type,
		entry.Description,
		entry.Amount,
		entry.Date,
		category.ID,
	).
		RunWith(d.db).
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

	entry.ID = lastInsertedID
	return nil
}

func (d *defaultEntryDao) UpdateEntry(entry *model.Entry) error {
	if entry == nil || entry.ID == 0 {
		return fmt.Errorf("Can't update nil entry")
	}

	//Begin Transaction
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}

	// Update database
	category, err := d.CreateCategoryIfNotExists(&model.Category{
		Name:      entry.Category.ValueOrZero(),
		AccountID: entry.AccountID,
		Type:      entry.Type,
	})
	if err != nil {
		return err
	}

	res, err := sq.
		Update("entry").
		Set("type", entry.Type).
		Set("description", entry.Description).
		Set("amount", entry.Amount).
		Set("date", entry.Date).
		Set("category", category.ID).
		Where(sq.And{
			sq.Eq{"id": entry.ID},
			sq.Eq{"account_id": entry.AccountID},
		}).
		RunWith(d.db).
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
		return fmt.Errorf("Entry not found: %q", entry.ID)
	}

	return nil
}

func (d *defaultEntryDao) FindCategoriesByName(names []string, accountId int64) ([]*model.Category, error) {

	rows, err := sq.Select(
		"id",
		"account_id",
		"name",
		"type",
	).
		From("category").
		Where(sq.And{
			sq.Eq{"account_id": accountId},
			sq.Eq{"name": names},
		}).
		RunWith(d.db).
		Query()

	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	if err == sql.ErrNoRows {
		return []*model.Category{}, nil
	}

	categories := make([]*model.Category, 0, len(names))
	for rows.Next() {
		var category model.Category
		if err := rows.Scan(
			&category.ID,
			&category.AccountID,
			&category.Name,
			&category.Type,
		); err != nil {
			return nil, err
		}
		categories = append(categories, &category)
	}

	return categories, nil
}

func (d *defaultEntryDao) CreateCategoriesIfNotExist(categories []*model.Category) ([]*model.Category, error) {
	if len(categories) == 0 {
		return []*model.Category{}, nil
	}

	accountID := categories[0].AccountID

	//Begin Transaction
	tx, err := d.db.Begin()
	if err != nil {
		return nil, err
	}

	categoryNames := make([]string, 0, len(categories))
	categoryTypeMap := make(map[string]model.Type)
	for _, category := range categories {
		categoryNames = append(categoryNames, category.Name)
		categoryTypeMap[category.Name] = category.Type
	}

	// (1) Fetch named categories
	existingCategories, err := d.FindCategoriesByName(categoryNames, accountID)
	if err != nil {
		return nil, err
	}

	// Find categories that do not exist
	newCategories := []*model.Category{}
	existingCategoriesMap := map[string]*model.Category{}

	for _, existingCategory := range existingCategories {
		existingCategoriesMap[fmt.Sprintf("%d-%s", existingCategory.Type, existingCategory.Name)] = existingCategory
	}

	for _, category := range categories {
		if _, found := existingCategoriesMap[fmt.Sprintf("%d-%s", category.Type, category.Name)]; !found {
			newCategories = append(newCategories, category)
		}
	}

	if len(newCategories) == 0 {
		return existingCategories, nil
	}

	// (2) Create new categories
	ib := sq.
		Insert("category").
		Columns(
			"account_id",
			"name",
			"type",
		)

	for _, newCategory := range newCategories {
		ib = ib.Values(newCategory.AccountID, newCategory.Name, newCategory.Type)
	}

	_, err = ib.RunWith(d.db).Exec()
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return nil, rollbackErr
		}
		return nil, err
	}

	// Commit
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	// (3) Query to get all categories
	return d.FindCategoriesByName(categoryNames, accountID)
}

func (d *defaultEntryDao) CreateCategoryIfNotExists(category *model.Category) (*model.Category, error) {

	if category == nil {
		return nil, fmt.Errorf("Cannot create category %q", category)
	}
	if !category.Type.IsIncome() && !category.Type.IsExpense() {
		return nil, fmt.Errorf("Category is neither income nor expense. Type %q", category.Type)
	}

	//Begin Transaction
	tx, err := d.db.Begin()
	if err != nil {
		return nil, err
	}

	rows, err := sq.Select(
		"id",
		"account_id",
		"name",
		"type",
	).
		From("category").
		Where(sq.And{
			sq.Eq{"account_id": category.AccountID},
			sq.Eq{"name": category.Name},
		}).
		Limit(1).
		RunWith(d.db).
		Query()

	if err != nil && err != sql.ErrNoRows {
		tx.Rollback()
		return nil, err
	}

	if rows.Next() {
		var existingCategory model.Category
		if err := rows.Scan(
			&existingCategory.ID,
			&existingCategory.AccountID,
			&existingCategory.Name,
			&existingCategory.Type,
		); err != nil {
			tx.Rollback()
			return nil, err
		}
		tx.Commit()
		return &existingCategory, nil
	}

	res, err := sq.
		Insert("category").
		Columns(
			"account_id",
			"name",
			"type",
		).
		Values(category.AccountID, category.Name, category.Type).
		RunWith(d.db).
		Exec()
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return nil, rollbackErr
		}
		return nil, err
	}

	// Commit
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	lastInsertedID, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &model.Category{
		ID:        lastInsertedID,
		AccountID: category.AccountID,
		Name:      category.Name,
		Type:      category.Type,
	}, nil
}

func (d *defaultEntryDao) DeleteEntries(ids []int64) (int64, error) {
	tx, err := d.db.Begin()
	if err != nil {
		return 0, err
	}

	res, err := sq.
		Delete("entry").
		Where(sq.And{
			sq.Eq{"id": ids},
		}).
		RunWith(d.db).
		Exec()

	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return 0, rollbackErr
		}
		return 0, err
	}

	// Commit transaction
	err = tx.Commit()
	if err := tx.Commit(); err != nil {
		return 0, err
	}

	rowsAffected, _ := res.RowsAffected()
	return rowsAffected, nil
}

func (d *defaultEntryDao) Categories(accountId int64) ([]*model.Category, error) {

	rows, err := sq.Select(
		"name",
		"type").
		From("category").
		Where(sq.Eq{"account_id": accountId}).
		OrderBy("name").
		RunWith(d.db).
		Query()

	if err != nil {
		return nil, err
	}

	categories := make([]*model.Category, 0)
	for rows.Next() {
		var category model.Category
		if err := rows.Scan(
			&category.Name,
			&category.Type,
		); err != nil {
			return nil, err
		}
		categories = append(categories, &category)
	}

	return categories, nil
}

func (d *defaultEntryDao) GetMininumAndMaximumExpenseForYear(year int) (*model.ExpenseRange, error) {

	rows, err := sq.Select(
		"MIN(amount) AS min_amount",
		"MAX(amount) AS max_amount").
		From("cumulative_amount").
		RunWith(d.db).
		Query()

	if err != nil {
		return nil, err
	}

	var fMinAmount float64
	var fMaxAmount float64

	if rows.Next() {
		if err := rows.Scan(
			&fMinAmount,
			&fMaxAmount,
		); err != nil {
			return nil, err
		}
	}

	minAmount, _ := decimal.NewFromString(fmt.Sprintf("%.3f", fMinAmount))
	maxAmount, _ := decimal.NewFromString(fmt.Sprintf("%.3f", fMaxAmount))

	return model.NewExpenseRange(
		minAmount,
		maxAmount,
	), nil
}

func (d *defaultEntryDao) GetMonthStartBalanceForYear(year int) ([]*model.ChartSeries, error) {

	rows, err := sq.Select(
		"account_id",
		"MONTH(CONCAT(month, \"-01\")) AS month",
		"amount",
	).
		From("cumulative_amount").
		Where(sq.Eq{"YEAR(CONCAT(month, \"-01\"))": year}).
		RunWith(d.db).
		Query()

	if err != nil {
		return nil, err
	}

	chartSeries := make([]*model.ChartSeries, 0, 12)
	for rows.Next() {
		var series model.ChartSeries
		if err := rows.Scan(
			&series.AccountID,
			&series.Month,
			&series.Amount,
		); err != nil {
			return nil, err
		}
		chartSeries = append(chartSeries, &series)
	}
	return chartSeries, nil
}

func (d *defaultEntryDao) GetTotalExpensePerCategoryForMonth(accountId int64, month int, categoryType model.Type) ([]*model.CategoryExpensesSummary, error) {

	rows, err := sq.Select(
		"e.account_id",
		"c.name AS category",
		"c.type",
		"c.id",
		"SUM(amount) AS amount",
	).
		From("entry e").
		LeftJoin("category c ON e.category = c.id").
		Where(sq.And{
			sq.Eq{"e.type": categoryType},
			sq.Eq{"e.accountId": accountId},
			sq.Eq{"MONTH(e.date)": month},
		}).
		RunWith(d.db).
		Query()

	if err != nil {
		return nil, err
	}

	expensesSummary := make([]*model.CategoryExpensesSummary, 0, 30)
	for rows.Next() {
		var accountId int64
		var categoryName string
		var categoryType model.Type
		var categoryId int64
		var fAmount float64
		if err := rows.Scan(
			&accountId,
			&categoryName,
			&categoryType,
			&categoryId,
			&fAmount,
		); err != nil {
			return nil, err
		}
		amount, _ := decimal.NewFromString(fmt.Sprintf("%.3f", fAmount))
		expensesSummary = append(expensesSummary, model.NewCategoryExpenseSummary(&model.Category{
			ID:        categoryId,
			AccountID: accountId,
			Name:      categoryName,
			Type:      categoryType,
		}, month, amount))
	}
	return expensesSummary, nil
}
