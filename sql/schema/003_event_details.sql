-- +goose Up

CREATE TABLE event_details (
    id UUID PRIMARY KEY,
    show_date TIMESTAMP NOT NULL,
    price NUMERIC(10, 2) NOT NULL DEFAULT 0.00,
    number_of_tickets INTEGER NOT NULL,
	ticket_description TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NULL,
	event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE
);

-- +goose Down

DROP TABLE event_details;
