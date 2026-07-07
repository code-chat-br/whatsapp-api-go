-- name: CreateMessage :one
INSERT INTO "Message" (
    "keyId",
    "keyRemoteJid",
    "keyLid",
    "keyFromMe",
    "keyParticipant",
    "keyParticipantLid",
    "pushName",
    "messageType",
    "content",
    "messageTimestamp",
    "device",
    "isGroup",
    "instanceId",
    "metadata"
) VALUES (
    @keyId,
    sqlc.narg('keyRemoteJid'),
    sqlc.narg('keyLid'),
    @keyFromMe,
    sqlc.narg('keyParticipant'),
    sqlc.narg('keyParticipantLid'),
    sqlc.narg('pushName'),
    @messageType,
    @content,
    @messageTimestamp,
    @device,
    sqlc.narg('isGroup'),
    @instanceId,
    sqlc.narg('metadata')
)
RETURNING *;

-- name: CreateMessageOrIgnore :exec
INSERT INTO "Message" (
    "keyId",
    "keyRemoteJid",
    "keyLid",
    "keyFromMe",
    "keyParticipant",
    "keyParticipantLid",
    "pushName",
    "messageType",
    "content",
    "messageTimestamp",
    "device",
    "isGroup",
    "instanceId",
    "metadata"
) VALUES (
    @keyId,
    sqlc.narg('keyRemoteJid'),
    sqlc.narg('keyLid'),
    @keyFromMe,
    sqlc.narg('keyParticipant'),
    sqlc.narg('keyParticipantLid'),
    sqlc.narg('pushName'),
    @messageType,
    @content,
    @messageTimestamp,
    @device,
    sqlc.narg('isGroup'),
    @instanceId,
    sqlc.narg('metadata')
)
ON CONFLICT ("instanceId", "keyId") DO NOTHING;

-- name: CountMessages :one
SELECT count(*)
FROM "Message" m
WHERE m."instanceId" = @instanceId
  AND (NOT @filterKeyID::boolean OR m."keyId" = @keyId)
  AND (NOT @filterKeyRemoteJid::boolean OR m."keyRemoteJid" = @keyRemoteJid)
  AND (NOT @filterKeyFromMe::boolean OR m."keyFromMe" = @keyFromMe)
  AND (NOT @filterMessageType::boolean OR m."messageType" = @messageType)
  AND (NOT @filterDevice::boolean OR m."device" = @device)
  AND (NOT @filterMessageTimestampGte::boolean OR m."messageTimestamp" >= @messageTimestampGte)
  AND (NOT @filterMessageTimestampLte::boolean OR m."messageTimestamp" <= @messageTimestampLte)
  AND (
      NOT @filterMessageStatus::boolean
      OR EXISTS (
          SELECT 1
          FROM "MessageUpdate" mu
          WHERE mu."messageId" = m."id"
            AND mu."status" = @messageStatus
      )
  );

-- name: FindMessageByIDForInstance :one
SELECT *
FROM "Message"
WHERE "instanceId" = @instanceId
  AND "id" = @id;

-- name: FindMessagesByIDsForInstance :many
SELECT *
FROM "Message"
WHERE "instanceId" = @instanceId
  AND "id" = ANY(@ids::int[])
ORDER BY "id";

-- name: FindMessageByKeyIDForInstance :one
SELECT *
FROM "Message"
WHERE "instanceId" = @instanceId
  AND "keyId" = @keyId
ORDER BY "id" DESC
LIMIT 1;

-- name: FindOutgoingMessageByIDForInstance :one
SELECT *
FROM "Message"
WHERE "instanceId" = @instanceId
  AND "id" = @id
  AND "keyFromMe" = true;

-- name: FindOutgoingMessageByKeyIDForInstance :one
SELECT *
FROM "Message"
WHERE "instanceId" = @instanceId
  AND "keyId" = @keyId
  AND "keyFromMe" = true
ORDER BY "id" DESC
LIMIT 1;

-- name: MarkMessagesReadForInstance :execrows
INSERT INTO "MessageUpdate" (
    "dateTime",
    "status",
    "messageId"
)
SELECT
    @dateTime,
    'READ',
    m."id"
FROM "Message" m
WHERE m."instanceId" = @instanceId
  AND m."id" = ANY(@ids::int[]);

-- name: UpdateMessageContentForInstance :one
UPDATE "Message"
SET "content" = @content
WHERE "instanceId" = @instanceId
  AND "id" = @id
RETURNING *;

-- name: ListMessagesNext :many
SELECT *
FROM "Message" m
WHERE m."instanceId" = @instanceId
  AND (NOT @hasCursor::boolean OR m."id" > @cursor)
  AND (NOT @filterKeyID::boolean OR m."keyId" = @keyId)
  AND (NOT @filterKeyRemoteJid::boolean OR m."keyRemoteJid" = @keyRemoteJid)
  AND (NOT @filterKeyFromMe::boolean OR m."keyFromMe" = @keyFromMe)
  AND (NOT @filterMessageType::boolean OR m."messageType" = @messageType)
  AND (NOT @filterDevice::boolean OR m."device" = @device)
  AND (NOT @filterMessageTimestampGte::boolean OR m."messageTimestamp" >= @messageTimestampGte)
  AND (NOT @filterMessageTimestampLte::boolean OR m."messageTimestamp" <= @messageTimestampLte)
  AND (
      NOT @filterMessageStatus::boolean
      OR EXISTS (
          SELECT 1
          FROM "MessageUpdate" mu
          WHERE mu."messageId" = m."id"
            AND mu."status" = @messageStatus
      )
  )
ORDER BY m."id"
LIMIT @limitCount;

-- name: ListMessagesPrevious :many
SELECT *
FROM "Message" m
WHERE m."instanceId" = @instanceId
  AND (NOT @hasCursor::boolean OR m."id" < @cursor)
  AND (NOT @filterKeyID::boolean OR m."keyId" = @keyId)
  AND (NOT @filterKeyRemoteJid::boolean OR m."keyRemoteJid" = @keyRemoteJid)
  AND (NOT @filterKeyFromMe::boolean OR m."keyFromMe" = @keyFromMe)
  AND (NOT @filterMessageType::boolean OR m."messageType" = @messageType)
  AND (NOT @filterDevice::boolean OR m."device" = @device)
  AND (NOT @filterMessageTimestampGte::boolean OR m."messageTimestamp" >= @messageTimestampGte)
  AND (NOT @filterMessageTimestampLte::boolean OR m."messageTimestamp" <= @messageTimestampLte)
  AND (
      NOT @filterMessageStatus::boolean
      OR EXISTS (
          SELECT 1
          FROM "MessageUpdate" mu
          WHERE mu."messageId" = m."id"
            AND mu."status" = @messageStatus
      )
  )
ORDER BY m."id" DESC
LIMIT @limitCount;

-- name: CountMessagesBeforeID :one
SELECT count(*)
FROM "Message" m
WHERE m."instanceId" = @instanceId
  AND m."id" < @id
  AND (NOT @filterKeyID::boolean OR m."keyId" = @keyId)
  AND (NOT @filterKeyRemoteJid::boolean OR m."keyRemoteJid" = @keyRemoteJid)
  AND (NOT @filterKeyFromMe::boolean OR m."keyFromMe" = @keyFromMe)
  AND (NOT @filterMessageType::boolean OR m."messageType" = @messageType)
  AND (NOT @filterDevice::boolean OR m."device" = @device)
  AND (NOT @filterMessageTimestampGte::boolean OR m."messageTimestamp" >= @messageTimestampGte)
  AND (NOT @filterMessageTimestampLte::boolean OR m."messageTimestamp" <= @messageTimestampLte)
  AND (
      NOT @filterMessageStatus::boolean
      OR EXISTS (
          SELECT 1
          FROM "MessageUpdate" mu
          WHERE mu."messageId" = m."id"
            AND mu."status" = @messageStatus
      )
  );

-- name: MessageExists :one
SELECT EXISTS (
    SELECT 1
    FROM "Message"
    WHERE "id" = @id
) AS "exists";
