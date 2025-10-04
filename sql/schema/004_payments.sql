-- +goose Up

CREATE TABLE payments (
    id UUID PRIMARY KEY,
    payment_intent_id TEXT NULL,
    amount NUMERIC(10, 2) NOT NULL DEFAULT 0.00,
	currency TEXT NOT NULL,
    status TEXT NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NULL,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE
);

-- +goose Down

DROP TABLE payments;