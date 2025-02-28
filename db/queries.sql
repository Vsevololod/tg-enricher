
-- name: GetVideoByID :one
SELECT * FROM video WHERE hash_id = @hash_id;

-- name: UpdateVideo :exec
UPDATE video
SET path        = @path,
    title       = @title,
    duration    = @duration,
    timestamp   = @timestamp,
    filesize    = @filesize,
    thumbnail   = @thumbnail,
    channel_url = @channel_url,
    channel_id  = @channel_id,
    channel     = @channel,
    video_id    = @video_id
WHERE hash_id = @hash_id;

