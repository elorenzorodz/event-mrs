-- +goose Up

ALTER TABLE event_details ADD COLUMN tickets_remaining INTEGER;

UPDATE event_details
SET tickets_remaining = number_of_tickets;

ALTER TABLE event_details 
    ALTER COLUMN tickets_remaining SET DEFAULT 0,
    ALTER COLUMN tickets_remaining SET NOT NULL;

-- +goose Down

ALTER TABLE event_details DROP COLUMN tickets_remaining;