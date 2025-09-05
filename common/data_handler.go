package common

import "database/sql"

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