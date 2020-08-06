package api

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/RadhiFadlillah/duit/internal/model"
	"github.com/jszwec/csvutil"
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

	entries, err := h.entryDao.Entries(int64(accountID), ForMonth(month, year))
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
	var ids []int64
	err := json.NewDecoder(r.Body).Decode(&ids)
	checkError(err)

	h.entryDao.DeleteEntries(ids)
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

	fileHeader := r.MultipartForm.File["import"][0]
	file, err := fileHeader.Open()
	checkError(err)
	defer file.Close()

	bytes, err := ioutil.ReadAll(file)
	checkError(err)

	// Read CSV File
	var csvEntries []struct {
		Date        string `csv:"Date"`
		Amount      string `csv:"Amount"`
		Category    string `csv:"Category"`
		Description string `csv:"Description"`
	}
	csvutil.Unmarshal(bytes, &csvEntries)
	checkError(err)

	entries := make([]*model.Entry, 0, len(csvEntries))
	errorBuilder := strings.Builder{}
	for i, entry := range csvEntries {

		addError := func(message string) {
			errorBuilder.WriteString(fmt.Sprintf("Entry %d (Date: %q, Description: %q): %s", i, entry.Date, entry.Description, message))
		}

		if len(entry.Date) == 0 || len(entry.Amount) == 0 {
			addError("'Date' and 'Amount' columns are mandatory")
			continue
		}

		date := ""
		acceptableLayouts := []string{"2006-01-02", "2006-1-2", "2006-Jan-02", "2006-Jan-2", "2/01/2006"}
		for _, layout := range acceptableLayouts {
			if tDate, err := time.Parse(layout, entry.Date); err == nil {
				date = tDate.Format("2006-01-02")
				break
			}
		}
		if len(date) == 0 {
			addError(fmt.Sprintf("Date must look like one of %s", acceptableLayouts))
			continue
		}

		var amount decimal.Decimal
		if amount, err = decimal.NewFromString(entry.Amount); err != nil {
			addError(fmt.Sprintf("Amount is not a decimal: %q", entry.Amount))
			continue
		}

		entryType := model.Income
		if amount.IsNegative() {
			entryType = model.Expense
			amount = amount.Abs()
		}

		category := null.String{}
		if len(entry.Category) != 0 {
			category = null.StringFrom(entry.Category)
		}

		description := null.String{}
		if len(entry.Description) != 0 {
			description = null.StringFrom(entry.Description)
		}

		entries = append(entries, &model.Entry{
			AccountID:         accountID,
			AffectedAccountID: affectedAccountID,
			Amount:            amount,
			Type:              entryType,
			Category:          category,
			Description:       description,
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
	fromDate, fromDateErr := time.Parse("2006-01-02", r.URL.Query().Get("fromDate"))
	toDate, toDateErr := time.Parse("2006-01-02", r.URL.Query().Get("toDate"))

	if fromDateErr != nil || toDateErr != nil {
		checkError(errors.New("fromDate and toDate must be formatted as yyyy-MM-dd"))
		return
	}

	entries, err := h.entryDao.Entries(int64(accountID), &TimeRange{fromDate, toDate})
	checkError(err)

	var exportCsvBuilder strings.Builder
	csvWriter := csv.NewWriter(&exportCsvBuilder)

	csvWriter.Write([]string{"Date", "Amount", "Category", "Description"})
	for _, entry := range entries {
		if entry.Type.IsExpense() {
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
