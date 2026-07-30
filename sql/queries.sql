-- name: GetURLByCode :one
SELECT * FROM urls
WHERE short_code = $1 LIMIT 1;

-- name: ListURL :many
SELECT 
    u.id, u.short_code, u.original_url, u.created_at,
    COALESCE(s.count, 0) AS click_count
FROM urls u
LEFT JOIN stats s ON u.id = s.url_id
ORDER BY s.last_updated DESC;

-- name: CreateURL :one
INSERT INTO urls (short_code, original_url)
VALUES ($1, $2)
RETURNING *;

-- name: UpsertStats :exec
INSERT INTO stats (url_id, count)
VALUES ($1, 1)
ON CONFLICT (url_id)
DO UPDATE SET 
    count = stats.count + 1,
    last_updated = NOW();

-- name: GetStatsByCode :one
SELECT 
    u.id, u.short_code, u.original_url, u.created_at,
    COALESCE(s.count, 0) AS click_count,
    s.last_updated
FROM urls u
LEFT JOIN stats s ON u.id = s.url_id
WHERE u.short_code = $1;