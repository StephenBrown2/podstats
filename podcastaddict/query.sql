-- name: UpdatePriority :exec
-- Update the priority of a podcast by its feed URL
UPDATE podcasts
SET priority = ?
WHERE feed_url = ?;

-- name: GetPodcastStats :many
-- Retrieve podcast statistics including unlistened episode count
SELECT p.name,
    p.author,
    p.feed_url,
    p.frequency,
    p.averageDuration AS average_duration,
    COALESCE(e.unplayed_count, 0) AS unplayed_episodes,
    p.priority
FROM podcasts p
    LEFT JOIN (
        SELECT podcast_id,
            COUNT(*) AS unplayed_count
        FROM episodes
        WHERE seen_status = 0 -- 0 = FALSE (not seen/unplayed)
        GROUP BY podcast_id
    ) e ON p._id = e.podcast_id
WHERE p.subscribed_status IS TRUE
ORDER BY p.priority DESC,
    p.name ASC;

-- name: GetPodcastStatsByCategory :many
-- Retrieve podcast statistics filtered by category
SELECT p.name,
    p.author,
    p.feed_url,
    p.frequency,
    p.averageDuration AS average_duration,
    COALESCE(e.unplayed_count, 0) AS unplayed_episodes,
    p.priority
FROM podcasts p
    LEFT JOIN (
        SELECT podcast_id,
            COUNT(*) AS unplayed_count
        FROM episodes
        WHERE seen_status = 0 -- 0 = FALSE (not seen/unplayed)
        GROUP BY podcast_id
    ) e ON p._id = e.podcast_id
WHERE p.subscribed_status IS TRUE
    AND p.category = ?
ORDER BY p.priority DESC,
    p.name ASC;

-- name: GetCategories :many
-- Get all unique categories from subscribed podcasts
SELECT DISTINCT p.category
FROM podcasts p
WHERE p.subscribed_status IS TRUE
    AND p.category IS NOT NULL
    AND p.category != ''
ORDER BY p.category ASC;
