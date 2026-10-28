-- name: CreateUser :one
INSERT INTO users (
    email,
    name,
    password_hash
) VALUES (
    @email,
    @name,
    @password_hash
) RETURNING id, email, name;

-- name: GetUserByEmail :one
SELECT
    id,
    password_hash
FROM users
WHERE email = @email;

-- name: CreateSession :one
INSERT INTO sessions (
    user_id,
    token_hash,
    expires_at
) VALUES (
    @user_id,
    @token_hash,
    @expires_at
) RETURNING id, user_id, expires_at;

-- name: CountRecentFailedLogins :one
SELECT count(*) FROM failed_logins
WHERE email = @email AND attempted_at > @since;

-- name: CreateFailedLogin :exec
INSERT INTO failed_logins (email) VALUES (@email);

-- name: DeleteFailedLogins :many
DELETE FROM failed_logins WHERE email = @email RETURNING id;

-- name: LockEmail :exec
SELECT pg_advisory_xact_lock(hashtext(@email));
