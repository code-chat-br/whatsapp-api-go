ALTER TABLE "Instance"
    ALTER COLUMN "connectionStatus" SET DEFAULT 'OFFLINE';

ALTER TYPE "InstanceStatus" RENAME TO "InstanceConnectionStatus";
