-- name: ListInventory :many
select i.*, coalesce(s.quantity, 0)::real as stock, s.cost_per_unit
from ingredients i left join inventory_items s on s.ingredient_id = i.id
order by i.category, i.name;

-- name: EnsureInventoryItem :one
insert into inventory_items (ingredient_id, quantity) values ($1, 0)
on conflict (ingredient_id) do update set ingredient_id = excluded.ingredient_id
returning *;

-- name: SetInventoryQuantity :exec
update inventory_items set quantity = $2, updated_at = now() where id = $1;
