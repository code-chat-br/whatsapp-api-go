CREATE TABLE "InstanceWhatsAppConnection" (
    "instanceId" integer PRIMARY KEY REFERENCES "Instance" ("id") ON DELETE CASCADE,
    "connectionStatus" varchar(64) NOT NULL DEFAULT 'offline',
    "whatsappDeviceJid" varchar(100) UNIQUE,
    "whatsappOwnerJid" varchar(100),
    "whatsappPhoneNumber" varchar(32),
    "profilePicId" varchar(255),
    "lastConnectedAt" timestamp,
    "lastDisconnectedAt" timestamp,
    "lastConnectionAttemptAt" timestamp,
    "lastConnectionError" varchar(255),
    "lastConnectionEvent" varchar(100),
    "connectionAttempts" integer NOT NULL DEFAULT 0,
    "createdAt" timestamp NOT NULL DEFAULT now(),
    "updatedAt" timestamp NOT NULL DEFAULT now()
);
