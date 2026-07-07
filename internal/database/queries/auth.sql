-- name: CreateAuth :one
INSERT INTO "Auth" (
    "token",
    "instanceId"
) VALUES (
    @token,
    @instanceId
)
RETURNING *;

-- name: FindAuthByInstanceID :one
SELECT *
FROM "Auth"
WHERE "instanceId" = @instanceId;

-- name: LockAuthByInstanceID :one
SELECT *
FROM "Auth"
WHERE "instanceId" = @instanceId
FOR UPDATE;

-- name: UpdateAuthToken :one
UPDATE "Auth"
SET
    "token" = @token,
    "updatedAt" = now()
WHERE "id" = @id
RETURNING *;

-- name: UpdateAuthTokenByInstanceAndOldToken :one
UPDATE "Auth"
SET
    "token" = @newToken,
    "updatedAt" = now()
WHERE "instanceId" = @instanceId
  AND "token" = @oldToken
RETURNING *;
