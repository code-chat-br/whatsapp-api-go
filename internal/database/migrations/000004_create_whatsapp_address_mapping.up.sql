CREATE TABLE "WhatsAppAddressMapping" (
    "instanceId" integer NOT NULL REFERENCES "Instance" ("id") ON DELETE CASCADE,
    "alias" varchar(100) NOT NULL,
    "normalizedPhone" varchar(32) NOT NULL,
    "canonicalJid" varchar(100) NOT NULL,
    "lidJid" varchar(100),
    "resolvedAt" timestamp NOT NULL,
    "expiresAt" timestamp NOT NULL,
    "createdAt" timestamp NOT NULL DEFAULT now(),
    "updatedAt" timestamp NOT NULL DEFAULT now(),
    PRIMARY KEY ("instanceId", "alias")
);

CREATE INDEX "WhatsAppAddressMapping_instanceId_canonicalJid_idx"
    ON "WhatsAppAddressMapping" ("instanceId", "canonicalJid");

CREATE INDEX "WhatsAppAddressMapping_instanceId_expiresAt_idx"
    ON "WhatsAppAddressMapping" ("instanceId", "expiresAt");

