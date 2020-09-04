package api

import (
	"github.com/julienschmidt/httprouter"
	"github.com/shopspring/decimal"
	"github.com/RadhiFadlillah/duit/internal/backend/utils"
	"net/http"
	"time"
)

// GetChartsData is handler for GET /api/charts
func (h *Handler) GetChartsData(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// Make sure session still valid
	h.auth.MustAuthenticateUser(r)

	// Get URL parameter
	year := utils.StrToInt(r.URL.Query().Get("year"))
	if year == 0 {
		year = time.Now().Year()
	}

	// Prepare statements
	accounts, err := h.accountDao.Accounts()
	checkError(err)

	chartSeries, err := h.entryDao.GetMonthStartBalanceForYear(year)
	checkError(err)

	chartLimit, err := h.entryDao.GetMininumAndMaximumExpenseForYear(year)
	checkError(err)

	// Calculate limit
	lenMaxAmount := len(chartLimit.MaxAmount().StringFixed(0))
	divisor := decimal.New(1, int32(lenMaxAmount-1))
	max := chartLimit.MaxAmount().Div(divisor).Ceil().Mul(divisor)
	min := chartLimit.MinAmount().Div(divisor).Ceil().Mul(divisor)

	// Return final result
	result := map[string]interface{}{
		"year":     year,
		"accounts": accounts,
		"series":   chartSeries,
		"min":      min,
		"max":      max,
	}

	w.Header().Add("Content-Encoding", "gzip")
	w.Header().Add("Content-Type", "application/json")
	err = encodeGzippedJSON(w, &result)
	checkError(err)
}
