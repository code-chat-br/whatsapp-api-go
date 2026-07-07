ALTER TABLE "Message"
    ADD COLUMN IF NOT EXISTS "metadata" jsonb;

WITH ranked_messages AS (
    SELECT
        "id",
        first_value("id") OVER (
            PARTITION BY "instanceId", "keyId"
            ORDER BY "id"
        ) AS keep_id,
        row_number() OVER (
            PARTITION BY "instanceId", "keyId"
            ORDER BY "id"
        ) AS rn
    FROM "Message"
),
duplicate_messages AS (
    SELECT "id", keep_id
    FROM ranked_messages
    WHERE rn > 1
)
UPDATE "MessageUpdate" mu
SET "messageId" = dm.keep_id
FROM duplicate_messages dm
WHERE mu."messageId" = dm."id";

WITH ranked_updates AS (
    SELECT
        "id",
        row_number() OVER (
            PARTITION BY "messageId", "status", "dateTime"
            ORDER BY "id"
        ) AS rn
    FROM "MessageUpdate"
)
DELETE FROM "MessageUpdate" mu
USING ranked_updates ru
WHERE mu."id" = ru."id"
  AND ru.rn > 1;

WITH ranked_messages AS (
    SELECT
        "id",
        row_number() OVER (
            PARTITION BY "instanceId", "keyId"
            ORDER BY "id"
        ) AS rn
    FROM "Message"
)
DELETE FROM "Message" m
USING ranked_messages rm
WHERE m."id" = rm."id"
  AND rm.rn > 1;

WITH ranked_contacts AS (
    SELECT
        "id",
        row_number() OVER (
            PARTITION BY "instanceId", "remoteJid"
            ORDER BY "updatedAt" DESC, "id" DESC
        ) AS rn
    FROM "Contact"
)
DELETE FROM "Contact" c
USING ranked_contacts rc
WHERE c."id" = rc."id"
  AND rc.rn > 1;

CREATE UNIQUE INDEX IF NOT EXISTS "Message_instanceId_keyId_key"
    ON "Message" ("instanceId", "keyId");

CREATE UNIQUE INDEX IF NOT EXISTS "Contact_instanceId_remoteJid_key"
    ON "Contact" ("instanceId", "remoteJid");

CREATE UNIQUE INDEX IF NOT EXISTS "MessageUpdate_messageId_status_dateTime_key"
    ON "MessageUpdate" ("messageId", "status", "dateTime");
