package api

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/RadhiFadlillah/duit/internal/model"
	"github.com/julienschmidt/httprouter"
	"github.com/shopspring/decimal"
	"gopkg.in/guregu/null.v3"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// SelectEntries is handler for GET /api/entries
func (h *Handler) SelectEntries(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// Make sure session still valid
	h.auth.MustAuthenticateUser(r)

	// Get URL parameter
	month := strToInt(r.URL.Query().Get("month"))
	year := strToInt(r.URL.Query().Get("year"))
	accountID := strToInt(r.URL.Query().Get("account"))

	entries, err := h.entryDao.Entries(int64(accountID), month, year)
	checkError(err)

	// Return final result
	result := map[string]interface{}{
		"entries": entries,
	}

	w.Header().Add("Content-Encoding", "gzip")
	w.Header().Add("Content-Type", "application/json")
	err = encodeGzippedJSON(w, &result)
	checkError(err)
}

// InsertEntry is handler for POST /api/entry
func (h *Handler) InsertEntry(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// Make sure session still valid
	h.auth.MustAuthenticateUser(r)

	// Decode request
	var entry model.Entry
	err := json.NewDecoder(r.Body).Decode(&entry)
	checkError(err)

	err = h.entryDao.SaveEntry(&entry)
	checkError(err)

	// Return inserted entry
	w.Header().Add("Content-Encoding", "gzip")
	w.Header().Add("Content-Type", "application/json")
	err = encodeGzippedJSON(w, &entry)
	checkError(err)
}

// UpdateEntry is handler for PUT /api/entry
func (h *Handler) UpdateEntry(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// Make sure session still valid
	h.auth.MustAuthenticateUser(r)

	// Decode request
	var entry model.Entry
	err := json.NewDecoder(r.Body).Decode(&entry)
	checkError(err)

	err = h.entryDao.UpdateEntry(&entry)
	checkError(err)

	// Return updated entry
	w.Header().Add("Content-Encoding", "gzip")
	w.Header().Add("Content-Type", "application/json")
	err = encodeGzippedJSON(w, &entry)
	checkError(err)
}

// DeleteEntries is handler for DELETE /api/entries
func (h *Handler) DeleteEntries(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// Make sure session still valid
	h.auth.MustAuthenticateUser(r)

	// Decode request
	var ids []int
	err := json.NewDecoder(r.Body).Decode(&ids)
	checkError(err)

	// Start transaction
	// Make sure to rollback if panic ever happened
	tx := h.db.MustBegin()

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	// Delete from database
	stmt, err := tx.Preparex(`DELETE FROM entry WHERE id = ?`)
	checkError(err)

	for _, id := range ids {
		stmt.MustExec(id)
	}

	// Commit transaction
	err = tx.Commit()
	checkError(err)
}

func (h *Handler) ImportEntriesFromCSV(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// Make sure session still valid
	h.auth.MustAuthenticateUser(r)

	// Parse Parameters
	err := r.ParseMultipartForm(32 << 20) // maxMemory 32MB
	checkError(err)

	// Validate Parameters
	if (len(r.MultipartForm.Value["accountID"])) == 0 {
		checkError(errors.New("account id not found"))
		return
	}

	if len(r.MultipartForm.File["import"]) == 0 {
		checkError(errors.New("Import file not found"))
		return
	}

	affectedAccountID := null.Int{}
	if len(r.MultipartForm.Value["affectedAccountID"]) != 0 {
		id, err := strconv.Atoi(r.MultipartForm.Value["affectedAccountID"][0])
		if err != nil {
			affectedAccountID = null.IntFrom(int64(id))
		}
	}

	var accountID int64
	iAccountID, err := strconv.Atoi(r.MultipartForm.Value["accountID"][0])
	checkError(err)
	accountID = int64(iAccountID)

	// Read CSV File
	fileHeader := r.MultipartForm.File["import"][0]
	file, err := fileHeader.Open()
	checkError(err)
	defer file.Close()

	bytes, err := ioutil.ReadAll(file)
	checkError(err)

	csvReader := csv.NewReader(strings.NewReader(string(bytes)))

	records, err := csvReader.ReadAll()
	checkError(err)

	dateIndex := -1
	amountIndex := -1
	descriptionIndex := -1
	categoryIndex := -1
	entries := make([]*model.Entry, 0, len(records))
	categories := make([]model.Category, 0, len(records))
	for i, record := range records {
		if i == 0 {
			for j, field := range record {

				if field == "Date" {
					dateIndex = j
				}
				if field == "Amount" {
					amountIndex = j
				}
				if field == "Description" {
					descriptionIndex = j
				}
				if field == "Category" {
					categoryIndex = j
				}
			}
			continue
		}

		if dateIndex == -1 || amountIndex == -1 {
			checkError(errors.New("'Date' and 'Amount' columns are mandatory"))
			return
		}

		description := null.String{}
		if descriptionIndex >= 0 {
			description = null.StringFrom(record[descriptionIndex])
		}

		var amount decimal.Decimal
		if amount, err = decimal.NewFromString(record[amountIndex]); err != nil {
			amount = decimal.NewFromInt(0)
		}

		entryType := 1 // Income
		if amount.IsNegative() {
			entryType = 2 // Expense
			amount = amount.Abs()
		}

		category := null.String{}
		if categoryIndex >= 0 {
			category = null.StringFrom(record[categoryIndex])
			categories = append(categories, model.Category{
				AccountID: accountID,
				Name:      category.ValueOrZero(),
				Type:      entryType,
			})
		}

		date := ""
		acceptableLayouts := []string{"2006-01-02", "2006-1-2", "2006-Jan-02", "2006-Jan-2", "2/01/2006"}
		for _, layout := range acceptableLayouts {
			if tDate, err := time.Parse(layout, record[dateIndex]); err == nil {
				date = tDate.Format("2006-01-02")
				break
			}
		}
		if len(date) == 0 {
			checkError(errors.New(fmt.Sprintf("Date must look like one of %s", acceptableLayouts)))
		}

		entries = append(entries, &model.Entry{
			AccountID:         accountID,
			AffectedAccountID: affectedAccountID,
			Type:              entryType,
			Description:       description,
			Category:          category,
			Amount:            amount,
			Date:              date,
		})
	}

	err = h.entryDao.SaveEntries(entries)
	checkError(err)

	// Return updated entry
	w.Header().Add("Content-Encoding", "gzip")
	w.Header().Add("Content-Type", "application/json")
	err = encodeGzippedJSON(w, entries)
	checkError(err)
}

func (h *Handler) ExportEntriesFromCSV(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// Make sure session still valid
	h.auth.MustAuthenticateUser(r)

	accountID := strToInt(r.URL.Query().Get("account"))

	// Start transaction
	tx := h.db.MustBegin()
	defer tx.Rollback()

	// Prepare SQL statement
	stmtSelectEntries, err := tx.Preparex(`
		SELECT e.id, e.account_id, e.affected_account_id,
			e.type, e.description, c.name as category, e.amount, e.date
		FROM entry e
		LEFT JOIN category c ON e.category = c.id
		WHERE e.account_id = ? 
		OR e.affected_account_id = ?
		ORDER BY e.date DESC, e.id DESC`)
	checkError(err)

	entries := []model.Entry{}
	err = stmtSelectEntries.Select(&entries,
		accountID, accountID)
	checkError(err)

	var exportCsvBuilder strings.Builder
	csvWriter := csv.NewWriter(&exportCsvBuilder)

	csvWriter.Write([]string{"Date", "Amount", "Category", "Description"})
	for _, entry := range entries {
		if entry.Type == 2 {
			entry.Amount = entry.Amount.Neg()
		}
		csvWriter.Write([]string{entry.Date, entry.Amount.String(), entry.Category.ValueOrZero(), entry.Description.ValueOrZero()})
	}
	csvWriter.Flush()

	w.Header().Set("Content-Disposition", "attachment; filename="+"export.csv")
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", exportCsvBuilder.Len()))

	io.Copy(w, strings.NewReader(exportCsvBuilder.String()))
}
