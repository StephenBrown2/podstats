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

-- name: GetTags :many
-- Get all tags from subscribed podcasts
SELECT DISTINCT t.name
FROM tags t
INNER JOIN tag_relation tr ON t._id = tr.tag_id
INNER JOIN podcasts p ON tr.podcast_id = p._id
WHERE p.subscribed_status IS TRUE
ORDER BY t.name ASC;

-- name: GetPodcastStatsByTag :many
-- Retrieve podcast statistics filtered by tag
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
    INNER JOIN tag_relation tr ON p._id = tr.podcast_id
    INNER JOIN tags t ON tr.tag_id = t._id
WHERE p.subscribed_status IS TRUE
    AND t.name = ?
ORDER BY p.priority DESC,
    p.name ASC;
