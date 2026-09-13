-- name: CreateInvite :one
insert into invites (token, email, created_by, created_at, expires_at)
values ($1, $2, $3, $4, $5) returning *;

-- name: ListInvites :many
select i.*, c.name as created_by_name, u.name as used_by_name
from invites i
join users c on c.id = i.created_by
left join users u on u.id = i.used_by
order by i.created_at desc;

-- name: FindValidInvite :one
select * from invites
where token = sqlc.arg(token) and used_at is null and revoked_at is null
  and expires_at > sqlc.arg(now);

-- name: ClaimInvite :one
update invites set used_at = sqlc.arg(now), used_by = sqlc.arg(used_by)
where token = sqlc.arg(token) and used_at is null and revoked_at is null
  and expires_at > sqlc.arg(now)
returning *;

-- name: RevokeInvite :one
update invites set revoked_at = sqlc.arg(now)
where id = sqlc.arg(id) and used_at is null and revoked_at is null
returning *;
