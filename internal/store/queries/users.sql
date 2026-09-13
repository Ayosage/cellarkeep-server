-- name: CountUsers :one
select count(*) from users;

-- name: CreateUser :one
insert into users (email, name, password_hash, role) values ($1, $2, $3, $4) returning *;

-- name: GetUserByEmail :one
select * from users where email = $1;

-- name: GetUser :one
select * from users where id = $1;

-- name: ListUsers :many
select id, email, name, role, created_at from users order by created_at;

-- name: UpdateUserPassword :exec
update users set password_hash = $2 where id = $1;
