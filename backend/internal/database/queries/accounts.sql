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

-- name: DeleteAccount :one
DELETE FROM accounts
WHERE id = @id
  AND user_id = @user_id
RETURNING
    id::text AS id,
    name,
    currency,
    balance_in_minor_units;

-- name: UpdateAccount :one
UPDATE accounts
SET
    name = COALESCE(sqlc.narg(name), name),
    currency = COALESCE(sqlc.narg(currency), currency),
    balance_in_minor_units = COALESCE(sqlc.narg(balance_in_minor_units), balance_in_minor_units)
WHERE id = @id
  AND user_id = @user_id
RETURNING
    id::text AS id,
    name,
    currency,
    balance_in_minor_units;
