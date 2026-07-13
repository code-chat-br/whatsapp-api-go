UPDATE "Instance" AS i
SET
    "connectionStatus" = 'ONLINE',
    "updatedAt" = now()
FROM "InstanceWhatsAppConnection" AS c
WHERE c."instanceId" = i."id"
  AND i."connectionStatus" = 'OFFLINE'
  AND c."connectionStatus" = 'logged_out';
