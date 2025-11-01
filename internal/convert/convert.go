package convert

import (
	"fmt"
	"math"
	"strconv"
	"time"
)

func StringToTime(dateTime string) (time.Time, string, error) {
	const referenceShowDateFormat = "2006-01-02 15:04"
	showDate, parseShowDateError := time.Parse(referenceShowDateFormat, dateTime)

	return showDate, referenceShowDateFormat, parseShowDateError
}

func StringToFloat32(number string) (float32, error) {
	price, err := strconv.ParseFloat(number, 32)

	if err != nil {
		return 0.0, fmt.Errorf("error parsing number '%s' to float32: %w", number, err)
	}

	return float32(price), nil
}

func PriceStringToCents(priceString string) (int64, error) {
	priceFloat, err := strconv.ParseFloat(priceString, 64)

	if err != nil {
		return 0, fmt.Errorf("error parsing price string: %w", err)
	}

	// Multiply by 100 and round to the nearest integer to handle money correctly.
	cents := math.Round(priceFloat * 100)

	return int64(cents), nil
}