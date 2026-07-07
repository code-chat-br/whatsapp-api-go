-- name: CreateContact :one
INSERT INTO "Contact" (
    "remoteJid",
    "pushName",
    "profilePicUrl",
    "instanceId"
) VALUES (
    @remoteJid,
    sqlc.narg('pushName'),
    sqlc.narg('profilePicUrl'),
    @instanceId
)
RETURNING *;

-- name: UpsertContact :one
INSERT INTO "Contact" (
    "remoteJid",
    "pushName",
    "profilePicUrl",
    "instanceId"
) VALUES (
    @remoteJid,
    sqlc.narg('pushName'),
    sqlc.narg('profilePicUrl'),
    @instanceId
)
ON CONFLICT ("instanceId", "remoteJid")
DO UPDATE SET
    "pushName" = COALESCE(NULLIF(EXCLUDED."pushName", ''), "Contact"."pushName"),
    "profilePicUrl" = COALESCE(NULLIF(EXCLUDED."profilePicUrl", ''), "Contact"."profilePicUrl"),
    "updatedAt" = now()
RETURNING *;

-- name: ListContacts :many
SELECT *
FROM "Contact"
WHERE "instanceId" = @instanceId
  AND (
      (@filterID::boolean AND "id" = @id)
      OR (
          NOT @filterID::boolean
          AND (NOT @filterRemoteJid::boolean OR "remoteJid" = @remoteJid)
          AND (NOT @filterPushName::boolean OR "pushName" = @pushName)
      )
  )
ORDER BY "id";
