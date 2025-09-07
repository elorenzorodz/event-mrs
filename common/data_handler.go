package common

import (
	"database/sql"
	"time"
)

func StringToNullString(text string) sql.NullString {
	if text == "" {
		return sql.NullString{Valid: false}
	}

	return sql.NullString{String: text, Valid: true}
}

func NullTimeToString(nullString sql.NullTime) string {
	if !nullString.Valid {
		return ""
	}

	return nullString.Time.String()
}

func StringToTime(dateTime string) (time.Time, string, error) {
	referenceShowDateFormat := "2006-01-02 15:04"
	showDate, parseShowDateError := time.Parse(referenceShowDateFormat, dateTime)

	return showDate, referenceShowDateFormat, parseShowDateError
}