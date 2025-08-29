package users

import "github.com/elorenzorodz/event-mrs/internal/database"

func DatabaseUserToUserAuthorizedJSON(databaseUser database.User, accessToken string) UserAuthorized {
	return UserAuthorized{
		Email: databaseUser.Email,
		AccessToken: accessToken,
	}
}