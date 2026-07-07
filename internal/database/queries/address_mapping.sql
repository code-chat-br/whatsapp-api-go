-- name: FindAddressMappingByAlias :one
SELECT *
FROM "WhatsAppAddressMapping"
WHERE "instanceId" = @instanceId
  AND "alias" = @alias
LIMIT 1;

-- name: ListAddressMappingAliases :many
SELECT "alias"
FROM "WhatsAppAddressMapping"
WHERE "instanceId" = @instanceId
  AND "canonicalJid" = @canonicalJid
ORDER BY "alias";

-- name: DeleteAddressMappingByCanonicalJID :execrows
DELETE FROM "WhatsAppAddressMapping"
WHERE "instanceId" = @instanceId
  AND "canonicalJid" = @canonicalJid;

-- name: UpsertAddressMappingAlias :execrows
INSERT INTO "WhatsAppAddressMapping" (
    "instanceId",
    "alias",
    "normalizedPhone",
    "canonicalJid",
    "lidJid",
    "resolvedAt",
    "expiresAt"
) VALUES (
    @instanceId,
    @alias,
    @normalizedPhone,
    @canonicalJid,
    sqlc.narg('lidJid'),
    @resolvedAt,
    @expiresAt
)
ON CONFLICT ("instanceId", "alias") DO UPDATE
SET
    "normalizedPhone" = @normalizedPhone,
    "canonicalJid" = @canonicalJid,
    "lidJid" = sqlc.narg('lidJid'),
    "resolvedAt" = @resolvedAt,
    "expiresAt" = @expiresAt,
    "updatedAt" = now();

