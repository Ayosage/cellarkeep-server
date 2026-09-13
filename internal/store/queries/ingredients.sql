-- name: CreateIngredient :one
insert into ingredients (name, category, unit) values ($1, $2, $3) returning *;

-- name: GetIngredient :one
select * from ingredients where id = $1;
