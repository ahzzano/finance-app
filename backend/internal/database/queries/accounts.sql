-- name: CreateAccount :one
INSERT INTO accounts (
    user_id,
    name,
    currency,
    balance_in_minor_units
) VALUES (
    @user_id,
    @name,
    @currency,
    @balance_in_minor_units
) RETURNING
    id::text AS id,
    name,
    currency,
    balance_in_minor_units;

-- name: ListAccountsByUser :many
SELECT
    id::text AS id,
    name,
    currency,
    balance_in_minor_units
FROM accounts
WHERE user_id = @user_id
ORDER BY created_at, id;
