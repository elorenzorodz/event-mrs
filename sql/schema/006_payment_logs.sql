-- +goose Up

CREATE TABLE payment_logs (
    id UUID PRIMARY KEY,
    status TEXT NOT NULL,
    description TEXT NULL,
    payment_intent_id TEXT NOT NULL,
    payment_method_id TEXT NULL,
    amount NUMERIC(10, 2) NOT NULL DEFAULT 0.00,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    user_email TEXT NOT NULL,
    payment_id UUID NOT NULL
);

-- +goose Down

DROP TABLE payment_logs;