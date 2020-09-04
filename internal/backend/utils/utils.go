package utils

import (
	"math/rand"
	"strconv"
)

const (
	capsLetters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	letters     = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"
)

func RandomString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}

	return string(b)
}

func StrToInt(str string) int {
	result, _ := strconv.Atoi(str)
	return result
}
