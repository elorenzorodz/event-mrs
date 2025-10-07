-- +goose Up

CREATE TABLE payment_logs (
    id UUID PRIMARY KEY,
    status STRING NOT NULL,
    description STRING NULL,
    payment_intent_id TEXT NOT NULL,
    payment_method_id TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    payment_id UUID NOT NULL REFERENCES payments(id) ON DELETE CASCADE
);

-- +goose Down

DROP TABLE payment_logs;