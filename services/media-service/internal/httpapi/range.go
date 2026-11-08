package httpapi

import (
	"errors"
	"strconv"
	"strings"
)

var errInvalidRange = errors.New("invalid byte range")

type byteRange struct {
	start int64
	end   int64
}

func parseByteRange(header string, size int64) (byteRange, bool, error) {
	if header == "" {
		return byteRange{}, false, nil
	}
	unit, value, found := strings.Cut(strings.TrimSpace(header), "=")
	if !found || !strings.EqualFold(strings.TrimSpace(unit), "bytes") ||
		value == "" || strings.Contains(value, ",") || size <= 0 {
		return byteRange{}, true, errInvalidRange
	}
	startValue, endValue, found := strings.Cut(strings.TrimSpace(value), "-")
	if !found || (startValue == "" && endValue == "") {
		return byteRange{}, true, errInvalidRange
	}

	if startValue == "" {
		suffixLength, err := parseRangeNumber(endValue)
		if err != nil || suffixLength == 0 {
			return byteRange{}, true, errInvalidRange
		}
		if suffixLength > size {
			suffixLength = size
		}
		return byteRange{start: size - suffixLength, end: size - 1}, true, nil
	}

	start, err := parseRangeNumber(startValue)
	if err != nil || start >= size {
		return byteRange{}, true, errInvalidRange
	}
	end := size - 1
	if endValue != "" {
		end, err = parseRangeNumber(endValue)
		if err != nil || start > end {
			return byteRange{}, true, errInvalidRange
		}
		if end >= size {
			end = size - 1
		}
	}
	return byteRange{start: start, end: end}, true, nil
}

func parseRangeNumber(value string) (int64, error) {
	if value == "" {
		return 0, errInvalidRange
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return 0, errInvalidRange
		}
	}
	result, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, errInvalidRange
	}
	return result, nil
}
