# event-mrs

Event Management and Reservation System is a simple API project which users can register and add, edit, and delete their events such as hosting movie shows, live bands, workshops and others. Sell tickets for the events and process Stripe payments.

## How to setup

```bash
go mod init github.com/elorenzorodz/event-mrs

go mod vendor

go mod tidy
```

## How to run

```bash
go build; .\event-mrs.exe

OR

go run .
```

## Requirements

- Stripe
- PostgreSQL
- [github.com/gin-gonic/gin](github.com/gin-gonic/gin)
- [github.com/joho/godotenv](github.com/joho/godotenv)
- [github.com/lib/pq](github.com/lib/pq)
- [github.com/google/uuid](github.com/google/uuid)
- [github.com/golang-jwt/jwt/v5](github.com/golang-jwt/jwt/v5)
- [golang.org/x/crypto](golang.org/x/crypto)
- [github.com/sqlc-dev/sqlc](https://github.com/sqlc-dev/sqlc)
- [github.com/pressly/goose](https://github.com/pressly/goose)
- [github.com/mailgun/mailgun-go/v4](github.com/mailgun/mailgun-go/v4)
