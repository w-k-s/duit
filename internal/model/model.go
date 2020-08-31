package model

import (
	"github.com/shopspring/decimal"
	"gopkg.in/guregu/null.v3"
)

type Type int

const (
	Income Type = iota + 1
	Expense
	Transfer
)

func (t Type) IsIncome() bool {
	return t == Income
}

func (t Type) IsExpense() bool {
	return t == Expense
}

func (t Type) IsTransfer() bool {
	return t == Transfer
}

// Config is content of configuration file
type Config struct {
	DbUser     string
	DbPassword string
	DbHost     string
	DbName     string
}

// Category is container for expense's category
type Category struct {
	ID        int64  `db:"id"         json:"-"`
	AccountID int64  `db:"account_id" json:"-"`
	Name      string `db:"name"       json:"name"`
	Type      Type   `db:"type"       json:"type"`
}

// User is container for user's data
type User struct {
	ID       int64  `db:"id"       json:"id"`
	Username string `db:"username" json:"username"`
	Name     string `db:"name"     json:"name"`
	Password string `db:"password" json:"password,omitempty"`
	Admin    bool   `db:"admin"    json:"admin"`
}

// Account is container for financial account
type Account struct {
	ID            int64           `db:"id"             json:"id"`
	Name          string          `db:"name"           json:"name"`
	InitialAmount decimal.Decimal `db:"initial_amount" json:"initialAmount"`

	// Additional fields that used in view
	Total decimal.Decimal `db:"total" json:"total"`
}

// Entry is container for book entries
type Entry struct {
	ID                int64           `db:"id"                  json:"id"`
	AccountID         int64           `db:"account_id"          json:"accountId"`
	AffectedAccountID null.Int        `db:"affected_account_id" json:"affectedAccountId"`
	Type              Type            `db:"type"                json:"type"`
	Description       null.String     `db:"description"         json:"description"`
	Category          null.String     `db:"category"            json:"category"`
	Amount            decimal.Decimal `db:"amount"              json:"amount"`
	Date              string          `db:"date"                json:"date"`

	// Additional foreign key fields
	Account         string      `db:"account"          json:"account"`
	AffectedAccount null.String `db:"affected_account" json:"affectedAccount"`
}

// ChartSeries is container for chart series
type ChartSeries struct {
	AccountID int64           `db:"account_id" json:"accountId"`
	Month     int             `db:"month"      json:"month"`
	Amount    decimal.Decimal `db:"amount"     json:"amount"`
}

type ExpenseRange struct {
	minAmount decimal.Decimal
	maxAmount decimal.Decimal
}

func NewExpenseRange(minAmount decimal.Decimal, maxAmount decimal.Decimal) *ExpenseRange {
	return &ExpenseRange{
		minAmount,
		maxAmount,
	}
}

func (er *ExpenseRange) MinAmount() decimal.Decimal {
	return er.minAmount
}

func (er *ExpenseRange) MaxAmount() decimal.Decimal {
	return er.maxAmount
}

type CategoryExpensesSummary struct {
	category *Category
	month    int
	expense  decimal.Decimal
}

func NewCategoryExpenseSummary(category *Category, month int, expense decimal.Decimal) *CategoryExpensesSummary {
	return &CategoryExpensesSummary{
		category,
		month,
		expense,
	}
}
