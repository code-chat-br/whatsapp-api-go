-- name: CreateMessageUpdate :one
INSERT INTO "MessageUpdate" (
    "dateTime",
    "status",
    "messageId"
) VALUES (
    @dateTime,
    @status,
    @messageId
)
RETURNING *;

-- name: CreateMessageUpdateOrIgnore :exec
INSERT INTO "MessageUpdate" (
    "dateTime",
    "status",
    "messageId"
) VALUES (
    @dateTime,
    @status,
    @messageId
)
ON CONFLICT ("messageId", "status", "dateTime") DO NOTHING;

-- name: ListMessageUpdatesByMessageID :many
SELECT *
FROM "MessageUpdate"
WHERE "messageId" = @messageId
ORDER BY "dateTime", "id";

-- name: ListMessageUpdatesByMessageIDs :many
SELECT *
FROM "MessageUpdate"
WHERE "messageId" = ANY(@messageIds::int[])
ORDER BY "messageId", "dateTime", "id";
