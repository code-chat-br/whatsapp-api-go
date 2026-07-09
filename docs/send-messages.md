# Envio de mensagens

Este documento descreve todos os endpoints de envio em `/message` e o recurso `options.mentionAll`.

Todas as rotas exigem o bearer token da instância:

```http
Authorization: Bearer <instance-token>
```

Rotas disponíveis:

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

`MessageOptions` é opcional. Quando `mentionAll` está ausente ou é `false`, o envio continua síncrono e retorna a mensagem persistida com `200 OK`.

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

`delay`: inteiro opcional em milissegundos. Envios gerais de mensagem aceitam até `120000`. Envios de áudio do WhatsApp aceitam até `300000`.

`presence`: string opcional. Texto, link, mídia, contato e localização aceitam `composing`. Áudio/PTV aceita `recording`. Áudio do WhatsApp também aceita `paused`.

`quotedMessageId`: id interno opcional da mensagem a ser citada. A mensagem precisa pertencer à mesma instância.

`quotedMessage`: snapshot opcional da mensagem citada com `keyId`, `keyRemoteJid`, `messageType` e `content`.

`externalAttributes`: objeto opcional copiado para os metadados da mensagem persistida e para os webhooks de resultado assíncrono de `mentionAll`.

`mentionAll`: booleano opcional. Quando `true`, o destinatário precisa ser um JID de grupo e a mensagem é aceita para processamento assíncrono quando o tipo da mensagem protobuf do WhatsApp suporta `ContextInfo`.

## Bodies dos endpoints

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

Suporta `mentionAll`.

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

Suporta `mentionAll`.

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

`mediatype` aceita `image`, `document`, `video`, `audio` e `ptv`. Suporta `mentionAll`.

### sendMediaFile

```http
POST /message/sendMediaFile/codechat
Content-Type: multipart/form-data
```

Campos multipart:

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

`attachment` é o campo do arquivo. `mediaType` aceita `image`, `document`, `video`, `audio` e `ptv`. Suporta `mentionAll`.

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

Baixa o áudio, converte/prepara como áudio PTT do WhatsApp e envia um `audioMessage`. Suporta `mentionAll`.

### sendWhatsAppAudioFile

```http
POST /message/sendWhatsAppAudioFile/codechat
Content-Type: multipart/form-data
```

Campos multipart:

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

`attachment` é o campo do arquivo de áudio. Suporta `mentionAll`.

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

`contactMessage` aceita um ou mais contatos. Se `vcard` for omitido, o serviço gera um a partir de `fullName`, `wuid`, `phoneNumber` e `organization`. Suporta `mentionAll`.

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

Suporta `mentionAll`.

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

Não suporta `mentionAll` porque `ReactionMessage` aponta para uma mensagem existente e não tem um campo `ContextInfo.MentionedJID` válido. Se `options.mentionAll=true`, a API retorna `400 Bad Request` com o código `MENTION_ALL_NOT_SUPPORTED_FOR_MESSAGE_TYPE`.

## Respostas de sucesso

Envios síncronos:

```http
HTTP/1.1 200 OK
Content-Type: application/json
```

O corpo da resposta é a linha de mensagem persistida retornada pelo serviço de mensagens. O webhook existente `send.message` é disparado depois da persistência.

Envios assíncronos com `mentionAll`:

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

`202 Accepted` significa apenas que a fila limitada aceitou o job. O resultado final é entregue pelo webhook existente `send.message` e correlacionado por `processId`.

## Menção invisível

`mentionAll=true` menciona todos os participantes atuais do grupo preenchendo `ContextInfo.MentionedJID` do WhatsApp. O servidor não adiciona marcadores visíveis `@phone` ao texto, legendas, cartões de contato, localizações ou corpos de mídia.

Endpoints suportados:

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

Endpoints não suportados:

```text
sendReaction
```

A lista de participantes é buscada quando o worker processa o job. Participantes que entrarem depois dessa busca não serão mencionados. Participantes que saírem durante o processamento ainda podem estar presentes na lista buscada.

## Resultado por webhook

O evento de webhook existente é reutilizado:

```text
send.message
```

Exemplo de sucesso:

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

Exemplo de falha:

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

Códigos de erro implementados para webhooks assíncronos:

```text
INSTANCE_NOT_CONNECTED
GROUP_INFO_FETCH_FAILED
GROUP_HAS_NO_PARTICIPANTS
MESSAGE_SEND_FAILED
GROUP_MENTION_PROCESSING_FAILED
```

## Erros HTTP

O destinatário não é um grupo:

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

O tipo de mensagem não suporta `mentionAll`:

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

A fila está cheia:

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

O processador está parado ou indisponível:

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

Outros erros de validação, autenticação, instância, mídia, upload, persistência e conexão com o WhatsApp mantêm o envelope padrão de erro da API.

## Comportamento da fila

Envios assíncronos usam uma fila limitada em memória gerenciada por `MessageProcessingManager`. A quantidade de workers e o tamanho da fila são fixados na inicialização. Se a fila estiver cheia, a requisição recebe `503` e nenhum job é aceito.

Workers são rastreados com um `sync.WaitGroup`. O shutdown para de aceitar novos jobs, fecha a fila, espera os workers e cancela o processamento pelo contexto de ciclo de vida da aplicação quando o deadline de shutdown é atingido.

## Variáveis de ambiente

```env
MESSAGE_PROCESSING_WORKERS="4"
MESSAGE_PROCESSING_QUEUE_SIZE="100"
MESSAGE_PROCESSING_TIMEOUT="60s"
MESSAGE_GROUP_INFO_TIMEOUT="30s"
MESSAGE_SEND_TIMEOUT="30s"
```

## Fluxo de processamento

```text
1. O cliente envia uma mensagem compatível com options.mentionAll=true.
2. A API valida autenticação, instância, payload e destinatário.
3. A API confirma que o destinatário é um JID de grupo.
4. A API cria processId.
5. O job é enviado para a fila limitada.
6. A API retorna HTTP 202 Accepted.
7. Um worker recarrega a instância e o cliente WhatsApp conectado.
8. O worker busca os participantes atuais do grupo.
9. Os JIDs dos participantes são deduplicados e adicionados a ContextInfo.MentionedJID.
10. O corpo visível original da mensagem é preservado.
11. A mensagem é enviada pelo whatsmeow.
12. O resultado final é publicado pelo webhook send.message.
```

## Limitações conhecidas

`mentionAll` funciona somente para JIDs de grupo com servidor `g.us`.

O servidor não adiciona marcadores visíveis `@phone`.

`sendReaction` rejeita `mentionAll` porque reações não carregam um `ContextInfo` válido no nível da mensagem.

Grupos muito grandes podem aumentar o tempo de processamento.

`202 Accepted` confirma apenas que o job entrou na fila.

O webhook é a fonte do resultado final.

Clientes WhatsApp podem exibir ou notificar menções invisíveis de formas diferentes.
