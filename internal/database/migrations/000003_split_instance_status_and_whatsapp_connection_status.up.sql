ALTER TYPE "InstanceConnectionStatus" RENAME TO "InstanceStatus";

ALTER TABLE "Instance"
    ALTER COLUMN "connectionStatus" SET DEFAULT 'ONLINE';

UPDATE "Instance"
SET "connectionStatus" = 'ONLINE'
WHERE "connectionStatus" = 'OFFLINE';
