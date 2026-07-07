-- name: CreateInstance :one
INSERT INTO "Instance" (
    "name",
    "description",
    "connectionStatus",
    "ownerJid",
    "profilePicUrl",
    "externalAttributes"
) VALUES (
    @name,
    sqlc.narg('description'),
    @connectionStatus,
    sqlc.narg('ownerJid'),
    sqlc.narg('profilePicUrl'),
    sqlc.narg('externalAttributes')
)
RETURNING *;

-- name: FindInstanceWithAuthByName :one
SELECT
    i."id",
    i."name",
    i."description",
    i."connectionStatus",
    i."ownerJid",
    i."profilePicUrl",
    i."createdAt",
    i."updatedAt",
    i."externalAttributes",
    c."connectionStatus" AS "whatsappConnectionStatus",
    c."whatsappDeviceJid" AS "whatsappDeviceJid",
    c."whatsappOwnerJid" AS "whatsappOwnerJid",
    c."whatsappPhoneNumber" AS "whatsappPhoneNumber",
    c."profilePicId" AS "profilePicId",
    c."lastConnectedAt" AS "lastConnectedAt",
    c."lastDisconnectedAt" AS "lastDisconnectedAt",
    c."lastConnectionAttemptAt" AS "lastConnectionAttemptAt",
    c."lastConnectionError" AS "lastConnectionError",
    c."lastConnectionEvent" AS "lastConnectionEvent",
    c."connectionAttempts" AS "connectionAttempts",
    a."id" AS "authId",
    a."token" AS "authToken",
    a."createdAt" AS "authCreatedAt",
    a."updatedAt" AS "authUpdatedAt",
    a."instanceId" AS "authInstanceId"
FROM "Instance" i
LEFT JOIN "InstanceWhatsAppConnection" c ON c."instanceId" = i."id"
LEFT JOIN "Auth" a ON a."instanceId" = i."id"
WHERE i."name" = @name;

-- name: ListInstancesWithAuth :many
SELECT
    i."id",
    i."name",
    i."description",
    i."connectionStatus",
    i."ownerJid",
    i."profilePicUrl",
    i."createdAt",
    i."updatedAt",
    i."externalAttributes",
    c."connectionStatus" AS "whatsappConnectionStatus",
    c."whatsappDeviceJid" AS "whatsappDeviceJid",
    c."whatsappOwnerJid" AS "whatsappOwnerJid",
    c."whatsappPhoneNumber" AS "whatsappPhoneNumber",
    c."profilePicId" AS "profilePicId",
    c."lastConnectedAt" AS "lastConnectedAt",
    c."lastDisconnectedAt" AS "lastDisconnectedAt",
    c."lastConnectionAttemptAt" AS "lastConnectionAttemptAt",
    c."lastConnectionError" AS "lastConnectionError",
    c."lastConnectionEvent" AS "lastConnectionEvent",
    c."connectionAttempts" AS "connectionAttempts",
    a."id" AS "authId",
    a."token" AS "authToken",
    a."createdAt" AS "authCreatedAt",
    a."updatedAt" AS "authUpdatedAt",
    a."instanceId" AS "authInstanceId"
FROM "Instance" i
LEFT JOIN "InstanceWhatsAppConnection" c ON c."instanceId" = i."id"
LEFT JOIN "Auth" a ON a."instanceId" = i."id"
ORDER BY i."id";

-- name: ListInstanceDetails :many
SELECT
    i."id",
    i."name",
    i."description",
    i."connectionStatus",
    i."ownerJid",
    i."profilePicUrl",
    i."createdAt",
    i."updatedAt",
    i."externalAttributes",
    c."connectionStatus" AS "whatsappConnectionStatus",
    c."whatsappDeviceJid" AS "whatsappDeviceJid",
    c."whatsappOwnerJid" AS "whatsappOwnerJid",
    c."whatsappPhoneNumber" AS "whatsappPhoneNumber",
    c."profilePicId" AS "profilePicId",
    c."lastConnectedAt" AS "lastConnectedAt",
    c."lastDisconnectedAt" AS "lastDisconnectedAt",
    c."lastConnectionAttemptAt" AS "lastConnectionAttemptAt",
    c."lastConnectionError" AS "lastConnectionError",
    c."lastConnectionEvent" AS "lastConnectionEvent",
    c."connectionAttempts" AS "connectionAttempts",
    a."id" AS "authId",
    a."token" AS "authToken",
    a."createdAt" AS "authCreatedAt",
    a."updatedAt" AS "authUpdatedAt",
    a."instanceId" AS "authInstanceId",
    w."id" AS "webhookId",
    w."enabled" AS "webhookEnabled",
    w."url" AS "webhookUrl",
    w."events" AS "webhookEvents",
    w."createdAt" AS "webhookCreatedAt",
    w."updatedAt" AS "webhookUpdatedAt",
    w."instanceId" AS "webhookInstanceId"
FROM "Instance" i
LEFT JOIN "InstanceWhatsAppConnection" c ON c."instanceId" = i."id"
LEFT JOIN "Auth" a ON a."instanceId" = i."id"
LEFT JOIN "Webhook" w ON w."instanceId" = i."id"
WHERE
    sqlc.arg('filterByName')::boolean = false
    OR i."name" ILIKE '%' || @name || '%'
ORDER BY i."createdAt" DESC;

-- name: FindInstanceDetailsByName :one
SELECT
    i."id",
    i."name",
    i."description",
    i."connectionStatus",
    i."ownerJid",
    i."profilePicUrl",
    i."createdAt",
    i."updatedAt",
    i."externalAttributes",
    c."connectionStatus" AS "whatsappConnectionStatus",
    c."whatsappDeviceJid" AS "whatsappDeviceJid",
    c."whatsappOwnerJid" AS "whatsappOwnerJid",
    c."whatsappPhoneNumber" AS "whatsappPhoneNumber",
    c."profilePicId" AS "profilePicId",
    c."lastConnectedAt" AS "lastConnectedAt",
    c."lastDisconnectedAt" AS "lastDisconnectedAt",
    c."lastConnectionAttemptAt" AS "lastConnectionAttemptAt",
    c."lastConnectionError" AS "lastConnectionError",
    c."lastConnectionEvent" AS "lastConnectionEvent",
    c."connectionAttempts" AS "connectionAttempts",
    w."id" AS "webhookId",
    w."enabled" AS "webhookEnabled",
    w."url" AS "webhookUrl",
    w."events" AS "webhookEvents",
    w."createdAt" AS "webhookCreatedAt",
    w."updatedAt" AS "webhookUpdatedAt",
    w."instanceId" AS "webhookInstanceId"
FROM "Instance" i
LEFT JOIN "InstanceWhatsAppConnection" c ON c."instanceId" = i."id"
LEFT JOIN "Webhook" w ON w."instanceId" = i."id"
WHERE i."name" = @name;

-- name: FindAutoConnectInstances :many
SELECT
    i."id",
    i."name",
    i."description",
    i."connectionStatus",
    i."ownerJid",
    i."profilePicUrl",
    i."createdAt",
    i."updatedAt",
    i."externalAttributes",
    c."connectionStatus" AS "whatsappConnectionStatus",
    c."whatsappDeviceJid" AS "whatsappDeviceJid",
    c."whatsappOwnerJid" AS "whatsappOwnerJid",
    c."whatsappPhoneNumber" AS "whatsappPhoneNumber",
    c."profilePicId" AS "profilePicId",
    c."lastConnectedAt" AS "lastConnectedAt",
    c."lastDisconnectedAt" AS "lastDisconnectedAt",
    c."lastConnectionAttemptAt" AS "lastConnectionAttemptAt",
    c."lastConnectionError" AS "lastConnectionError",
    c."lastConnectionEvent" AS "lastConnectionEvent",
    c."connectionAttempts" AS "connectionAttempts"
FROM "Instance" i
JOIN "InstanceWhatsAppConnection" c ON c."instanceId" = i."id"
WHERE c."connectionStatus" IN (
    'online',
    'connecting',
    'reconnecting',
    'connection_timeout',
    'connection_error',
    'keepalive_timeout'
)
  AND i."connectionStatus" = 'ONLINE'
  AND c."whatsappDeviceJid" IS NOT NULL
ORDER BY i."id";

-- name: UpdateInstance :one
UPDATE "Instance"
SET
    "name" = CASE WHEN @setName::boolean THEN @name ELSE "name" END,
    "description" = CASE WHEN @setDescription::boolean THEN sqlc.narg('description') ELSE "description" END,
    "profilePicUrl" = CASE WHEN @setProfilePicUrl::boolean THEN sqlc.narg('profilePicUrl') ELSE "profilePicUrl" END,
    "externalAttributes" = CASE WHEN @setExternalAttributes::boolean THEN sqlc.narg('externalAttributes') ELSE "externalAttributes" END,
    "updatedAt" = now()
WHERE "id" = @id
RETURNING *;

-- name: UpdateInstanceStatus :execrows
UPDATE "Instance"
SET
    "connectionStatus" = @status,
    "updatedAt" = now()
WHERE "id" = @id;

-- name: UpdateInstanceConnectionState :execrows
INSERT INTO "InstanceWhatsAppConnection" (
    "instanceId",
    "connectionStatus",
    "lastConnectedAt",
    "lastDisconnectedAt",
    "lastConnectionAttemptAt",
    "lastConnectionError",
    "lastConnectionEvent",
    "connectionAttempts"
) VALUES (
    @id,
    CASE WHEN @setConnectionStatus::boolean THEN @connectionStatus::text ELSE 'offline' END,
    sqlc.narg('lastConnectedAt'),
    sqlc.narg('lastDisconnectedAt'),
    sqlc.narg('lastConnectionAttemptAt'),
    sqlc.narg('lastConnectionError'),
    sqlc.narg('lastConnectionEvent'),
    CASE WHEN @incrementAttempts::boolean THEN 1 ELSE 0 END
)
ON CONFLICT ("instanceId") DO UPDATE
SET
    "connectionStatus" = CASE WHEN @setConnectionStatus::boolean THEN @connectionStatus::text ELSE "InstanceWhatsAppConnection"."connectionStatus" END,
    "lastConnectedAt" = CASE WHEN @setLastConnectedAt::boolean THEN sqlc.narg('lastConnectedAt') ELSE "InstanceWhatsAppConnection"."lastConnectedAt" END,
    "lastDisconnectedAt" = CASE WHEN @setLastDisconnectedAt::boolean THEN sqlc.narg('lastDisconnectedAt') ELSE "InstanceWhatsAppConnection"."lastDisconnectedAt" END,
    "lastConnectionAttemptAt" = CASE WHEN @setLastConnectionAttemptAt::boolean THEN sqlc.narg('lastConnectionAttemptAt') ELSE "InstanceWhatsAppConnection"."lastConnectionAttemptAt" END,
    "lastConnectionError" = CASE WHEN @setLastConnectionError::boolean THEN sqlc.narg('lastConnectionError') ELSE "InstanceWhatsAppConnection"."lastConnectionError" END,
    "lastConnectionEvent" = CASE WHEN @setLastConnectionEvent::boolean THEN sqlc.narg('lastConnectionEvent') ELSE "InstanceWhatsAppConnection"."lastConnectionEvent" END,
    "connectionAttempts" = CASE
        WHEN @resetAttempts::boolean THEN 0
        WHEN @incrementAttempts::boolean THEN "InstanceWhatsAppConnection"."connectionAttempts" + 1
        ELSE "InstanceWhatsAppConnection"."connectionAttempts"
    END,
    "updatedAt" = now();

-- name: SaveWhatsAppDevice :execrows
WITH update_instance AS (
    UPDATE "Instance"
    SET
        "ownerJid" = @whatsappOwnerJid,
        "updatedAt" = now()
    WHERE "id" = @id
)
INSERT INTO "InstanceWhatsAppConnection" (
    "instanceId",
    "whatsappDeviceJid",
    "whatsappOwnerJid",
    "whatsappPhoneNumber"
) VALUES (
    @id,
    @whatsappDeviceJid,
    @whatsappOwnerJid,
    @whatsappPhoneNumber
)
ON CONFLICT ("instanceId") DO UPDATE
SET
    "whatsappDeviceJid" = @whatsappDeviceJid,
    "whatsappOwnerJid" = @whatsappOwnerJid,
    "whatsappPhoneNumber" = @whatsappPhoneNumber,
    "updatedAt" = now();

-- name: ClearWhatsAppDevice :execrows
WITH update_instance AS (
    UPDATE "Instance"
    SET
        "ownerJid" = NULL,
        "updatedAt" = now()
    WHERE "id" = @id
)
UPDATE "InstanceWhatsAppConnection"
SET
    "whatsappDeviceJid" = NULL,
    "whatsappOwnerJid" = NULL,
    "whatsappPhoneNumber" = NULL,
    "updatedAt" = now()
WHERE "instanceId" = @id;

-- name: UpdateProfilePicture :execrows
WITH update_instance AS (
    UPDATE "Instance"
    SET
        "profilePicUrl" = sqlc.narg('profilePicUrl'),
        "updatedAt" = now()
    WHERE "id" = @id
)
INSERT INTO "InstanceWhatsAppConnection" (
    "instanceId",
    "profilePicId"
) VALUES (
    @id,
    sqlc.narg('profilePicId')
)
ON CONFLICT ("instanceId") DO UPDATE
SET
    "profilePicId" = sqlc.narg('profilePicId'),
    "updatedAt" = now();

-- name: TryAcquireInstanceConnectionLock :one
SELECT pg_try_advisory_lock(hashtext(@instanceId));

-- name: ReleaseInstanceConnectionLock :one
SELECT pg_advisory_unlock(hashtext(@instanceId));

-- name: LockInstanceByID :one
SELECT "id"
FROM "Instance"
WHERE "id" = @id
FOR UPDATE;

-- name: DeleteInstance :execrows
DELETE FROM "Instance"
WHERE "id" = @id;

-- name: CountInstanceDependencies :one
SELECT
    (SELECT count(*) FROM "Message" m WHERE m."instanceId" = $1) AS "messages",
    (SELECT count(*) FROM "Chat" c WHERE c."instanceId" = $1) AS "chats",
    (SELECT count(*) FROM "Contact" ct WHERE ct."instanceId" = $1) AS "contacts",
    (SELECT count(*) FROM "Webhook" w WHERE w."instanceId" = $1) AS "webhooks";

-- name: InstanceExists :one
SELECT EXISTS (
    SELECT 1
    FROM "Instance"
    WHERE "id" = @id
) AS "exists";
