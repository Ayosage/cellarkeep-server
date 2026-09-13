-- name: ListVessels :many
select * from vessels order by created_at, name;

-- name: GetVessel :one
select * from vessels where id = $1;

-- name: CreateVessel :one
insert into vessels (name, kind, capacity_l, notes) values ($1, $2, $3, $4) returning *;

-- name: UpdateVessel :one
update vessels set name = $2, kind = $3, capacity_l = $4, notes = $5 where id = $1 returning *;

-- name: DeleteVessel :exec
delete from vessels where id = $1;

-- name: CountActiveBatchesInVessel :one
select count(*) from batches where current_vessel_id = $1 and status in ('fermenting','conditioning');
