-- name: GetURLByCode :one
SELECT * FROM links
WHERE code = $1 LIMIT 1;

-- name: ListURL :many
SELECT 
    link.id, link.code, link.target_url, link.title, link.created_at,
    COALESCE(stat.click_count, 0) AS click_count
FROM links link
LEFT JOIN link_stats stat ON link.id = stat.link_id
ORDER BY stat.last_clicked_at DESC, link.id DESC
LIMIT sqlc.arg('limit')
OFFSET sqlc.arg('offset');

-- name: CreateURL :one
INSERT INTO links (code, target_url, title)
VALUES (sqlc.arg('code'), sqlc.arg('target_url'), sqlc.narg('title'))
RETURNING *;

-- name: UpdateURL :one
UPDATE links
SET target_url = sqlc.arg('target_url'),
    title = sqlc.narg('title')
WHERE code = sqlc.arg('code')
RETURNING *;

-- name: DeleteURL :execrows
DELETE FROM links
WHERE code = $1;

-- name: RecordClick :exec
INSERT INTO link_stats (link_id, click_count)
VALUES ($1, 1)
ON CONFLICT (link_id)
DO UPDATE SET 
    click_count = link_stats.click_count + 1,
    last_clicked_at = NOW();

-- name: GetLinkStats :one
SELECT 
    link.id, link.code, link.target_url, link.title, link.created_at,
    COALESCE(stat.click_count, 0) AS click_count,
    stat.last_clicked_at
FROM links link
LEFT JOIN link_stats stat ON link.id = stat.link_id
WHERE link.code = $1;