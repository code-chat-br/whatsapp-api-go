DROP INDEX IF EXISTS "MessageUpdate_messageId_status_dateTime_key";
DROP INDEX IF EXISTS "Contact_instanceId_remoteJid_key";
DROP INDEX IF EXISTS "Message_instanceId_keyId_key";

ALTER TABLE "Message"
    DROP COLUMN IF EXISTS "metadata";
