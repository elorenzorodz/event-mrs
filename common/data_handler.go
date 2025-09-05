package common

import "database/sql"

func StringToNullString(text string) sql.NullString {
	if text == "" {
		return sql.NullString{Valid: false}
	}

	return sql.NullString{String: text, Valid: true}
}