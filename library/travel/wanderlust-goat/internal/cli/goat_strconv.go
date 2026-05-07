package cli

import (
	"errors"
	"strconv"
)

func strconvParseFloat(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

func strErr(msg string) error { return errors.New(msg) }

func strFmt(f float64) string { return strconv.FormatFloat(f, 'f', 6, 64) }
