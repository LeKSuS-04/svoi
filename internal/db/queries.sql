-- name: CreateUser :exec
INSERT INTO users (id, displayed_name)
VALUES (?, ?);

-- name: UpdateUser :exec
UPDATE users
SET displayed_name = ?
WHERE id = ?;

-- name: GetUser :one
SELECT displayed_name
FROM users
WHERE id = ?
LIMIT 1;

-- name: InitStat :exec
INSERT OR IGNORE INTO stats (user_id, chat_id)
VALUES (?, ?);

-- name: GetChatStats :many
SELECT user_id, svo_count, zov_count, likvidirovan_count
FROM stats
WHERE chat_id = ?
ORDER BY svo_count + zov_count DESC;

-- name: AddStats :exec
UPDATE stats
SET
    svo_count = svo_count + ?,
    zov_count = zov_count + ?,
    likvidirovan_count = likvidirovan_count + ?
WHERE
    user_id = ? AND chat_id = ?;
