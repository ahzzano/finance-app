-- +goose Up
CREATE TABLE failed_logins (
    id BIGSERIAL PRIMARY KEY,
    email TEXT NOT NULL,
    attempted_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX failed_logins_email_attempted_at_idx ON failed_logins (email, attempted_at);

-- +goose Down
DROP INDEX failed_logins_email_attempted_at_idx;
DROP TABLE failed_logins;
