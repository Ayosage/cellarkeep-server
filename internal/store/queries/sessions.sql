-- name: CreateSession :exec
insert into sessions (id, user_id, expires_at) values ($1, $2, $3);

-- name: GetSessionWithUser :one
select s.id, s.expires_at, u.id as user_id, u.role
from sessions s join users u on u.id = s.user_id
where s.id = $1 and s.expires_at > now();

-- name: TouchSession :exec
update sessions set expires_at = $2 where id = $1;

-- name: DeleteSession :exec
delete from sessions where id = $1;

-- name: DeleteUserSessions :exec
delete from sessions where user_id = $1;
