-- name: CreateChat :one
INSERT INTO "Chat" (
    "remoteJid",
    "content",
    "instanceId"
) VALUES (
    @remoteJid,
    sqlc.narg('content'),
    @instanceId
)
RETURNING *;

-- name: ListChats :many
SELECT *
FROM "Chat"
WHERE "instanceId" = @instanceId
  AND (
      @chatType = ''
      OR (@chatType = 'group' AND right("remoteJid", 5) = '@g.us')
      OR (@chatType = 'chats' AND right("remoteJid", 5) <> '@g.us')
  )
ORDER BY "id";
