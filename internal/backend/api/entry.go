package api

import (
	"database/sql"
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
	page := strToInt(r.URL.Query().Get("page"))
	accountID := strToInt(r.URL.Query().Get("account"))

	// Start transaction
	// We only use it to fetch the data,
	// so just rollback it later
	tx := h.db.MustBegin()
	defer tx.Rollback()

	// Prepare SQL statement
	stmtGetAccount, err := tx.Preparex(`SELECT id FROM account WHERE id = ?`)
	checkError(err)

	stmtGetEntriesMaxPage, err := tx.Preparex(`
		SELECT CEIL(COUNT(*) / ?) FROM entry
		WHERE account_id = ?
		OR affected_account_id = ?`)
	checkError(err)

	stmtSelectEntries, err := tx.Preparex(`
		SELECT e.id, e.account_id, e.affected_account_id,
			a1.name account, a2.name affected_account,
			e.type, e.description, e.amount, e.date
		FROM entry e
		LEFT JOIN account a1 ON e.account_id = a1.id
		LEFT JOIN account a2 ON e.affected_account_id = a2.id
		WHERE e.account_id = ?
		OR e.affected_account_id = ?
		ORDER BY e.date DESC, e.id DESC
		LIMIT ? OFFSET ?`)
	checkError(err)

	// Make sure account exist
	var tmpID int64
	err = stmtGetAccount.Get(&tmpID, accountID)
	checkError(err)

	if err == sql.ErrNoRows {
		panic(fmt.Errorf("account doesn't exist"))
	}

	// Get entry count and calculate max page
	var maxPage int
	err = stmtGetEntriesMaxPage.Get(&maxPage, pageLength,
		accountID, accountID)
	checkError(err)

	if page == 0 {
		page = 1
	} else if page > maxPage {
		page = maxPage
	}

	offset := (page - 1) * pageLength

	// Fetch entries from database
	entries := []model.Entry{}
	err = stmtSelectEntries.Select(&entries,
		accountID, accountID,
		pageLength, offset)
	checkError(err)

	// Return final result
	result := map[string]interface{}{
		"page":    page,
		"maxPage": maxPage,
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

	// Start transaction
	// Make sure to rollback if panic ever happened
	tx := h.db.MustBegin()

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	// Prepare statements
	stmtInsertEntry, err := tx.Preparex(`INSERT INTO entry 
		(account_id, affected_account_id, type, description, amount, date)
		VALUES (?, ?, ?, ?, ?, ?)`)
	checkError(err)

	stmtGetEntry, err := tx.Preparex(`
		SELECT e.id, e.account_id, e.affected_account_id,
			a1.name account, a2.name affected_account,
			e.type, e.description, e.amount, e.date
		FROM entry e
		LEFT JOIN account a1 ON e.account_id = a1.id
		LEFT JOIN account a2 ON e.affected_account_id = a2.id
		WHERE e.id = ?`)
	checkError(err)

	// Save to database
	res := stmtInsertEntry.MustExec(
		entry.AccountID,
		entry.AffectedAccountID,
		entry.Type,
		entry.Description,
		entry.Amount,
		entry.Date)
	entry.ID, _ = res.LastInsertId()

	// Fetch the inserted data
	err = stmtGetEntry.Get(&entry, entry.ID)
	checkError(err)

	// Commit transaction
	err = tx.Commit()
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

	// Start transaction
	// Make sure to rollback if panic ever happened
	tx := h.db.MustBegin()

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	// Prepare statements
	stmtUpdateEntry, err := tx.Preparex(`UPDATE entry 
		SET affected_account_id = ?, description = ?, amount = ?, date = ?
		WHERE id = ?`)
	checkError(err)

	stmtGetEntry, err := tx.Preparex(`
		SELECT e.id, e.account_id, e.affected_account_id,
			a1.name account, a2.name affected_account,
			e.type, e.description, e.amount, e.date
		FROM entry e
		LEFT JOIN account a1 ON e.account_id = a1.id
		LEFT JOIN account a2 ON e.affected_account_id = a2.id
		WHERE e.id = ?`)
	checkError(err)

	// Update database
	stmtUpdateEntry.MustExec(
		entry.AffectedAccountID, entry.Description,
		entry.Amount, entry.Date, entry.ID)

	// Fetch the updated data
	err = stmtGetEntry.Get(&entry, entry.ID)
	checkError(err)

	// Commit transaction
	err = tx.Commit()
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

	err := r.ParseMultipartForm(32 << 20) // maxMemory 32MB
	checkError(err)

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
	entries := make([]model.Entry, 0, len(records))
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
			}
			continue
		}

		if dateIndex == -1 || amountIndex == -1 {
			fmt.Fprint(w, "'Date' and 'Amount' columns are mandatory", http.StatusBadRequest)
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

		entries = append(entries, model.Entry{
			AccountID:         accountID,
			AffectedAccountID: affectedAccountID,
			Type:              entryType,
			Description:       description,
			Amount:            amount,
			Date:              date,
		})
	}

	// Start transaction
	// Make sure to rollback if panic ever happened
	tx := h.db.MustBegin()

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	valueStrings := make([]string, 0, len(entries))
	valueArgs := make([]interface{}, 0, len(entries)*6)
	for _, entry := range entries {
		valueStrings = append(valueStrings, "(?, ?, ?, ?, ?, ?)")
		valueArgs = append(valueArgs, entry.AccountID)
		valueArgs = append(valueArgs, entry.AffectedAccountID)
		valueArgs = append(valueArgs, entry.Type)
		valueArgs = append(valueArgs, entry.Description)
		valueArgs = append(valueArgs, entry.Amount)
		valueArgs = append(valueArgs, entry.Date)
	}
	stmt := fmt.Sprintf("INSERT INTO entry (account_id, affected_account_id, type, description, amount, date) VALUES %s",
		strings.Join(valueStrings, ","))
	_, err = h.db.Exec(stmt, valueArgs...)
	checkError(err)

	// Commit transaction
	err = tx.Commit()
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
			e.type, e.description, e.amount, e.date
		FROM entry e
		WHERE e.account_id = ?
		OR e.affected_account_id = ?
		ORDER BY e.date DESC, e.id DESC`)
	checkError(err)

	entries := []model.Entry{}
	err = stmtSelectEntries.Select(&entries,
		accountID, accountID)
	checkError(err)

	fmt.Printf("entries; %v", entries)
	var exportCsvBuilder strings.Builder
	csvWriter := csv.NewWriter(&exportCsvBuilder)

	csvWriter.Write([]string{"Date", "Amount", "Description"})
	for _, entry := range entries {
		csvWriter.Write([]string{entry.Date, entry.Amount.String(), entry.Description.ValueOrZero()})
	}
	csvWriter.Flush()

	w.Header().Set("Content-Disposition", "attachment; filename="+"export.csv")
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", exportCsvBuilder.Len()))

	io.Copy(w, strings.NewReader(exportCsvBuilder.String()))
}
