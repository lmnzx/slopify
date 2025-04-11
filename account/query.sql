-- name: CreateUser :one
INSERT INTO users (
    first_name, last_name, email, password
) VALUES (
    $1, $2, $3, $4
)
RETURNING *;

-- name: GetUser :one
SELECT * FROM users
WHERE email = $1;

-- name: DeleteUser :exec
DELETE FROM users
WHERE email = $1;
