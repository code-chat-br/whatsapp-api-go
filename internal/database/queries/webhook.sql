-- name: CreateWebhook :one
INSERT INTO "Webhook" (
    "url",
    "enabled",
    "events",
    "instanceId"
) VALUES (
    @url,
    @enabled,
    sqlc.narg('events'),
    @instanceId
)
RETURNING *;

-- name: FindWebhookByInstanceName :one
SELECT w.*
FROM "Webhook" w
JOIN "Instance" i ON i."id" = w."instanceId"
WHERE i."name" = @instanceName;

-- name: ListEnabledWebhooksWithInstance :many
SELECT
    w."id",
    w."url",
    w."enabled",
    w."events",
    w."createdAt",
    w."updatedAt",
    w."instanceId",
    i."name" AS "instanceName"
FROM "Webhook" w
JOIN "Instance" i ON i."id" = w."instanceId"
WHERE w."enabled" = true
ORDER BY w."id";

-- name: UpdateWebhook :one
UPDATE "Webhook"
SET
    "url" = CASE WHEN @setURL::boolean THEN @url ELSE "url" END,
    "enabled" = CASE WHEN @setEnabled::boolean THEN @enabled ELSE "enabled" END,
    "updatedAt" = now()
WHERE "id" = @id
RETURNING *;

-- name: MergeWebhookEvents :one
UPDATE "Webhook"
SET
    "events" = COALESCE("events", '{}'::jsonb) || @events::jsonb,
    "updatedAt" = now()
WHERE "id" = @id
RETURNING *;

-- name: ClearWebhookEvents :one
UPDATE "Webhook"
SET
    "events" = '{}'::jsonb,
    "updatedAt" = now()
WHERE "id" = @id
RETURNING *;
