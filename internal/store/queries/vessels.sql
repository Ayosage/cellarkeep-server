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

-- name: ListVesselsWithOccupant :many
select v.*, b.id as batch_id, b.name as batch_name, b.beverage as batch_beverage,
  b.volume_l as batch_volume_l, b.start_date as batch_start_date,
  r.completed_date as racked_in
from vessels v
left join batches b on b.current_vessel_id = v.id and b.status in ('fermenting','conditioning')
left join lateral (
  select e.completed_date from batch_events e
  where e.batch_id = b.id and e.type = 'racking' and e.status = 'done' and e.vessel_to_id = v.id
  order by e.completed_date desc nulls last
  limit 1
) r on true
order by v.created_at, v.name;
