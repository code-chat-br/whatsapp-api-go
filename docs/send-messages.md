# Send messages

This document describes all `/message` sending endpoints and the `options.mentionAll` feature.

All routes require the instance bearer token:

```http
Authorization: Bearer <instance-token>
```

Available routes:

```text
POST /message/sendText/:instanceName
POST /message/sendLink/:instanceName
POST /message/sendMedia/:instanceName
POST /message/sendMediaFile/:instanceName
POST /message/sendWhatsAppAudio/:instanceName
POST /message/sendWhatsAppAudioFile/:instanceName
POST /message/sendContact/:instanceName
POST /message/sendLocation/:instanceName
POST /message/sendReaction/:instanceName
```

## MessageOptions

`MessageOptions` is optional. When `mentionAll` is absent or `false`, the send remains synchronous and returns the persisted message with `200 OK`.

```json
{
  "delay": 1000,
  "presence": "composing",
  "quotedMessageId": 123,
  "quotedMessage": {
    "keyId": "A5FDD9082F21LGHLKJLGB6C3FF6BFA6F",
    "keyRemoteJid": "120363000000000000@g.us",
    "keyFromMe": false,
    "messageType": "extendedTextMessage",
    "content": {}
  },
  "externalAttributes": {
    "requestId": "request-456"
  },
  "mentionAll": true
}
```

`delay`: optional integer in milliseconds. General message sends accept up to `120000`. WhatsApp audio sends accept up to `300000`.

`presence`: optional string. Text, link, media, contact and location accept `composing`. Audio/PTV accepts `recording`. WhatsApp audio also accepts `paused`.

`quotedMessageId`: optional internal message id to quote. The message must belong to the same instance.

`quotedMessage`: optional quoted message snapshot with `keyId`, `keyRemoteJid`, `messageType`, and `content`.

`externalAttributes`: optional object copied into persisted message metadata and asynchronous `mentionAll` result webhooks.

`mentionAll`: optional boolean. When `true`, the recipient must be a group JID and the message is accepted for asynchronous processing when the WhatsApp protobuf message type supports `ContextInfo`.

## Endpoint Bodies

### sendText

```http
POST /message/sendText/codechat
Content-Type: application/json
```

```json
{
  "number": "120363000000000000@g.us",
  "options": {
    "mentionAll": true,
    "presence": "composing",
    "delay": 1000
  },
  "textMessage": {
    "text": "Aviso importante para todos."
  }
}
```

Supports `mentionAll`.

### sendLink

```http
POST /message/sendLink/codechat
Content-Type: application/json
```

```json
{
  "number": "5531999999999",
  "options": {
    "presence": "composing"
  },
  "linkMessage": {
    "link": "https://example.com",
    "thumbnailUrl": "https://example.com/thumb.jpg",
    "title": "Example",
    "description": "Example link"
  }
}
```

Supports `mentionAll`.

### sendMedia

```http
POST /message/sendMedia/codechat
Content-Type: application/json
```

```json
{
  "number": "5531999999999",
  "options": {
    "presence": "composing"
  },
  "mediaMessage": {
    "mediatype": "image",
    "fileName": "image.jpg",
    "caption": "Caption",
    "media": "https://example.com/image.jpg"
  }
}
```

`mediatype` accepts `image`, `document`, `video`, `audio`, and `ptv`. Supports `mentionAll`.

### sendMediaFile

```http
POST /message/sendMediaFile/codechat
Content-Type: multipart/form-data
```

Multipart fields:

```json
{
  "number": "5531999999999",
  "mediaType": "image",
  "caption": "Caption",
  "presence": "composing",
  "delay": "1200",
  "quotedMessageId": "123",
  "quotedMessage": "{\"keyId\":\"abc\",\"keyRemoteJid\":\"5531999999999@s.whatsapp.net\",\"messageType\":\"extendedTextMessage\",\"content\":{\"text\":\"quoted\"}}",
  "mentionAll": "false",
  "attachment": "<binary file>"
}
```

`attachment` is the file field. `mediaType` accepts `image`, `document`, `video`, `audio`, and `ptv`. Supports `mentionAll`.

### sendWhatsAppAudio

```http
POST /message/sendWhatsAppAudio/codechat
Content-Type: application/json
```

```json
{
  "number": "5531999999999",
  "options": {
    "presence": "recording"
  },
  "audioMessage": {
    "audio": "https://example.com/audio.mp3"
  }
}
```

Downloads audio, converts/prepares it as WhatsApp PTT audio, and sends an `audioMessage`. Supports `mentionAll`.

### sendWhatsAppAudioFile

```http
POST /message/sendWhatsAppAudioFile/codechat
Content-Type: multipart/form-data
```

Multipart fields:

```json
{
  "number": "5531999999999",
  "presence": "recording",
  "delay": "1200",
  "quotedMessageId": "123",
  "quotedMessage": "{\"keyId\":\"abc\",\"keyRemoteJid\":\"5531999999999@s.whatsapp.net\",\"messageType\":\"extendedTextMessage\",\"content\":{\"text\":\"quoted\"}}",
  "mentionAll": "false",
  "attachment": "<binary audio file>"
}
```

`attachment` is the audio file field. Supports `mentionAll`.

### sendContact

```http
POST /message/sendContact/codechat
Content-Type: application/json
```

```json
{
  "number": "5531999999999",
  "options": {
    "quotedMessageId": 123,
    "presence": "composing"
  },
  "contactMessage": [
    {
      "fullName": "Code Chat",
      "wuid": "5531999999999@s.whatsapp.net",
      "phoneNumber": "+55 31 99999-9999",
      "organization": "CodeChat",
      "vcard": "BEGIN:VCARD\nVERSION:3.0\nFN:Code Chat\nTEL;type=CELL;waid=5531999999999:+55 31 99999-9999\nEND:VCARD"
    }
  ]
}
```

`contactMessage` accepts one or more contacts. If `vcard` is omitted, the service generates one from `fullName`, `wuid`, `phoneNumber`, and `organization`. Supports `mentionAll`.

### sendLocation

```http
POST /message/sendLocation/codechat
Content-Type: application/json
```

```json
{
  "number": "5531999999999",
  "options": {
    "presence": "composing"
  },
  "locationMessage": {
    "name": "Belo Horizonte",
    "address": "Minas Gerais",
    "url": "https://example.com/place",
    "latitude": -19.9212,
    "longitude": -43.9378
  }
}
```

Supports `mentionAll`.

### sendReaction

```http
POST /message/sendReaction/codechat
Content-Type: application/json
```

```json
{
  "reactionMessage": {
    "key": {
      "remoteJid": "5531999999999@s.whatsapp.net",
      "fromMe": true,
      "id": "3EB0FDD9082F21A9AC3D"
    },
    "reaction": "ok"
  }
}
```

Does not support `mentionAll` because `ReactionMessage` targets an existing message and has no valid `ContextInfo.MentionedJID` field. If `options.mentionAll=true`, the API returns `400 Bad Request` with code `MENTION_ALL_NOT_SUPPORTED_FOR_MESSAGE_TYPE`.

## Success Responses

Synchronous sends:

```http
HTTP/1.1 200 OK
Content-Type: application/json
```

The response body is the persisted message row returned by the message service. The existing `send.message` webhook is dispatched after persistence.

Asynchronous `mentionAll` sends:

```http
HTTP/1.1 202 Accepted
Content-Type: application/json
```

```json
{
  "statusCode": 202,
  "status": "processing",
  "message": "A mensagem foi aceita e esta sendo processada.",
  "processId": "019f4ec1-f9b1-7c33-a4ef-d47715cb29e4",
  "instanceName": "codechat"
}
```

`202 Accepted` means only that the bounded queue accepted the job. The final result is delivered by the existing `send.message` webhook and correlated by `processId`.

## Ghost mention

`mentionAll=true` mentions all current group participants by filling WhatsApp `ContextInfo.MentionedJID`. The server does not add visible `@phone` markers to text, captions, contact cards, locations, or media bodies.

Supported endpoints:

```text
sendText
sendLink
sendMedia
sendMediaFile
sendWhatsAppAudio
sendWhatsAppAudioFile
sendContact
sendLocation
```

Unsupported endpoints:

```text
sendReaction
```

The participant list is fetched when the worker processes the job. Participants who join after that fetch are not mentioned. Participants who leave during processing may still be present in the fetched list.

## Webhook result

The existing webhook event is reused:

```text
send.message
```

Success example:

```json
{
  "event": "send.message",
  "instance": {
    "id": 1,
    "name": "codechat",
    "connectionStatus": "online",
    "ownerJid": "5511999999999@s.whatsapp.net",
    "externalAttributes": {}
  },
  "data": {
    "processId": "019f4ec1-f9b1-7c33-a4ef-d47715cb29e4",
    "status": "sent",
    "mentionAll": true,
    "data": {
      "messageId": "3EB0FDD9082F21A9AC3D",
      "remoteJid": "120363000000000000@g.us",
      "participantCount": 84,
      "timestamp": "2026-07-07T15:00:00Z"
    },
    "externalAttributes": {
      "requestId": "request-456"
    }
  },
  "timestamp": "2026-07-07T15:00:01Z"
}
```

Failure example:

```json
{
  "event": "send.message",
  "instance": {
    "id": 1,
    "name": "codechat",
    "connectionStatus": "online",
    "ownerJid": "5511999999999@s.whatsapp.net",
    "externalAttributes": {}
  },
  "data": {
    "processId": "019f4ec1-f9b1-7c33-a4ef-d47715cb29e4",
    "status": "failed",
    "mentionAll": true,
    "error": {
      "code": "GROUP_MENTION_PROCESSING_FAILED",
      "message": "Nao foi possivel concluir o envio da mensagem para o grupo."
    },
    "externalAttributes": {
      "requestId": "request-456"
    }
  },
  "timestamp": "2026-07-07T15:00:01Z"
}
```

Implemented asynchronous webhook error codes:

```text
INSTANCE_NOT_CONNECTED
GROUP_INFO_FETCH_FAILED
GROUP_HAS_NO_PARTICIPANTS
MESSAGE_SEND_FAILED
GROUP_MENTION_PROCESSING_FAILED
```

## HTTP errors

Recipient is not a group:

```json
{
  "statusCode": 400,
  "error": "bad-request",
  "code": "MENTION_ALL_REQUIRES_GROUP",
  "messages": [
    "A opcao mentionAll somente pode ser utilizada em grupos."
  ]
}
```

Message type does not support `mentionAll`:

```json
{
  "statusCode": 400,
  "error": "bad-request",
  "code": "MENTION_ALL_NOT_SUPPORTED_FOR_MESSAGE_TYPE",
  "messages": [
    "A opcao mentionAll nao e suportada para este tipo de mensagem."
  ]
}
```

Queue is full:

```json
{
  "statusCode": 503,
  "error": "service-unavailable",
  "code": "MESSAGE_PROCESSING_QUEUE_FULL",
  "messages": [
    "O servico de processamento de mensagens esta temporariamente ocupado."
  ]
}
```

Processor is stopped or unavailable:

```json
{
  "statusCode": 503,
  "error": "service-unavailable",
  "code": "MESSAGE_PROCESSOR_STOPPED",
  "messages": [
    "O servico de processamento de mensagens nao esta disponivel."
  ]
}
```

Other validation, auth, instance, media, upload, persistence, and WhatsApp connection errors keep the API standard error envelope.

## Queue behavior

Asynchronous sends use a bounded in-memory queue managed by `MessageProcessingManager`. The worker count and queue size are fixed at startup. If the queue is full, the request receives `503` and no job is accepted.

Workers are tracked with a `sync.WaitGroup`. Shutdown stops accepting new jobs, closes the queue, waits for workers, and cancels processing through the application lifecycle context when the shutdown deadline is reached.

## Environment variables

```env
MESSAGE_PROCESSING_WORKERS="4"
MESSAGE_PROCESSING_QUEUE_SIZE="100"
MESSAGE_PROCESSING_TIMEOUT="60s"
MESSAGE_GROUP_INFO_TIMEOUT="30s"
MESSAGE_SEND_TIMEOUT="30s"
```

## Processing flow

```text
1. The client sends a compatible message with options.mentionAll=true.
2. The API validates authentication, instance, payload and recipient.
3. The API confirms the recipient is a group JID.
4. The API creates processId.
5. The job is submitted to the bounded queue.
6. The API returns HTTP 202 Accepted.
7. A worker reloads the instance and connected WhatsApp client.
8. The worker fetches current group participants.
9. Participant JIDs are deduplicated and added to ContextInfo.MentionedJID.
10. The original visible message body is preserved.
11. The message is sent through whatsmeow.
12. The final result is published through the send.message webhook.
```

## Known limitations

`mentionAll` works only for group JIDs with server `g.us`.

The server does not add visible `@phone` markers.

`sendReaction` rejects `mentionAll` because reactions do not carry a valid message-level `ContextInfo`.

Very large groups may increase processing time.

`202 Accepted` confirms only that the job entered the queue.

The webhook is the source of the final result.

WhatsApp clients may display or notify ghost mentions differently.
