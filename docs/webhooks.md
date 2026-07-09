# Webhooks

Documentação técnica dos webhooks implementados no código executável atual.

## Sumário
- [Visão geral](#visão-geral)
- [Configuração](#configuração)
- [Mapa de eventos](#mapa-de-eventos)
- [Cabeçalhos HTTP](#cabeçalhos-http)
- [Envelope padrão](#envelope-padrão)
- [Estrutura da instância](#estrutura-da-instância)
- [Entrega e tratamento de erros](#entrega-e-tratamento-de-erros)
- [Eventos](#eventos)
- [Eventos não suportados ou ignorados](#eventos-não-suportados-ou-ignorados)

## Visão geral

Webhooks são requisições HTTP `POST` assíncronas enviadas pela aplicação para uma URL configurada pelo consumidor. Cada entrega contém um envelope comum com o nome externo do evento, a instância que originou o evento, os dados específicos de `data` e o `timestamp` de criação do webhook.

Existem dois destinos possíveis. O webhook da instância é configurado por `PUT /webhook/set/:instanceName`, fica no cache em memória e só recebe eventos cujas flags estejam habilitadas em `events`. O webhook global é configurado por `WEBHOOK_GLOBAL_URL` e `WEBHOOK_GLOBAL_ENABLED`; quando habilitado, recebe todos os eventos suportados, sem aplicar as flags da instância.

O cache é carregado na inicialização a partir dos webhooks habilitados e é atualizado quando a configuração da instância muda. Se a instância não tiver webhook habilitado e o webhook global estiver desabilitado, o evento é descartado sem erro.

As entregas entram em uma fila em memória e são processadas por múltiplos workers. Respostas HTTP `2xx` são sucesso; erros de rede, timeout e respostas não `2xx` são registrados em log. Não há retry automático, dead-letter queue ou garantia de ordenação entre eventos. Como há concorrência de workers, eventos da mesma instância podem ser entregues fora da ordem em que foram enfileirados.

- Versão do documento: `1.0.0`.
- Eventos oficiais documentados: `27`.
- Pacote de constantes: `internal/database/types/webhook.go`.
- Dispatcher: `internal/webhook/manager.go`.
- Versão auditada do whatsmeow: `v0.0.0-20260630180629-b572e5bcb92b`.

## Configuração

### Variáveis de ambiente

| Variável | Tipo | Padrão | Descrição |
| --- | --- | --- | --- |
| `WEBHOOK_GLOBAL_URL` | URL | vazio | URL HTTP ou HTTPS do webhook global. Obrigatória quando `WEBHOOK_GLOBAL_ENABLED=true`. |
| `WEBHOOK_GLOBAL_ENABLED` | boolean | `false` | Habilita o webhook global. Aceita `true`, `false` ou vazio. |

### Webhook da instância

Configurar ou atualizar:

```http
PUT /webhook/set/codechat HTTP/1.1
Authorization: Bearer <token-da-instancia>
Content-Type: application/json
```

Consultar:

```http
GET /webhook/find/codechat HTTP/1.1
Authorization: Bearer <token-da-instancia>
```

Objeto de configuração da instância:

```json
{
  "enabled": true,
  "events": {
    "callUpsert": true,
    "chatsDeleted": true,
    "chatsUpdated": true,
    "connectionUpdated": true,
    "contactsUpdated": true,
    "contactsUpsert": true,
    "groupsParticipantsUpdated": true,
    "groupsUpdated": true,
    "groupsUpsert": true,
    "historySync": true,
    "identityUpdated": true,
    "labelsAssociation": true,
    "labelsEdit": true,
    "mediaRetry": true,
    "messagesDeleted": true,
    "messagesStarred": true,
    "messagesUndecryptable": true,
    "messagesUpdated": true,
    "messagesUpsert": true,
    "newsLetter": true,
    "presenceUpdated": true,
    "profilePictureUpdated": true,
    "qrcodeUpdated": true,
    "sendMessage": true,
    "settingsUpdated": true,
    "statusInstance": true,
    "userAboutUpdated": true
  },
  "url": "https://example.com/webhooks/codechat"
}
```

`url` precisa usar `http` ou `https` e ter no máximo 500 caracteres. `enabled` ausente assume `true` na criação/atualização. Quando `events` é omitido, as flags existentes são preservadas; quando `events` é `{}`, as flags são removidas. Campos desconhecidos em `events` são rejeitados.

## Mapa de eventos

| Flag | Evento externo | Descrição |
| --- | --- | --- |
| `callUpsert` | `call.upsert` | Atualizacao de chamada de voz ou video. |
| `chatsDeleted` | `chats.delete` | Exclusao ou limpeza de conversa. |
| `chatsUpdated` | `chats.updated` | Atualizacao de propriedades de conversas. |
| `connectionUpdated` | `connection.update` | Mudanca de estado da conexao da instancia. |
| `contactsUpdated` | `contacts.update` | Atualizacao parcial em contato existente. |
| `contactsUpsert` | `contacts.upsert` | Contato criado ou atualizado no cadastro local. |
| `groupsParticipantsUpdated` | `groups.participants.update` | Mudanca de participantes em grupo. |
| `groupsUpdated` | `groups.update` | Atualizacao parcial de metadados de grupo. |
| `groupsUpsert` | `groups.upsert` | Grupo criado, descoberto ou sincronizado. |
| `historySync` | `history.sync` | Sincronizacao de historico recebida do WhatsApp. |
| `identityUpdated` | `identity.update` | Mudanca de identidade criptografica de um contato. |
| `labelsAssociation` | `labels.association` | Associacao ou remocao de label em chat ou mensagem. |
| `labelsEdit` | `labels.edit` | Criacao, alteracao ou remocao de label. |
| `mediaRetry` | `media.retry` | Resultado ou erro relacionado a tentativa de retry de midia. |
| `messagesDeleted` | `messages.delete` | Mensagem removida localmente pelo evento DeleteForMe. |
| `messagesStarred` | `messages.star` | Marcacao ou desmarcacao de estrela em mensagem. |
| `messagesUndecryptable` | `messages.undecryptable` | Mensagem recebida sem possibilidade de descriptografia. |
| `messagesUpdated` | `messages.update` | Atualizacao de recibo/status de uma mensagem ja conhecida. |
| `messagesUpsert` | `messages.upsert` | Mensagem recebida e persistida pela aplicacao. |
| `newsLetter` | `news.letter` | Eventos relacionados a newsletters/canais. |
| `presenceUpdated` | `presence.updated` | Atualizacao de presenca de usuario ou presenca em chat. |
| `profilePictureUpdated` | `profile.picture.update` | Atualizacao de foto de perfil da propria instancia ou de outro JID. |
| `qrcodeUpdated` | `qrcode.updated` | Novo QR Code disponivel para pareamento da instancia. |
| `sendMessage` | `send.message` | Mensagem enviada pela API apos envio e persistencia bem-sucedidos. |
| `settingsUpdated` | `settings.update` | Atualizacao de configuracoes do usuario/instancia. |
| `statusInstance` | `status.instance` | Eventos de estado operacional ou avisos da instancia. |
| `userAboutUpdated` | `user.about.update` | Atualizacao do recado/about de um usuario. |

## Cabeçalhos HTTP

| Cabeçalho | Exemplo | Descrição |
| --- | --- | --- |
| `Content-Type` | `application/json` | Formato do payload. |
| `User-Agent` | `CodeChat-Webhook/1.0` | Identifica o emissor do webhook. |
| `x-request-id` | `UUID ou request id do contexto` | Id de rastreio da entrega; nao e uma chave de idempotencia garantida. |
| `x-owner-jid` | `JID do proprietário ou string vazia` | Proprietário da instância quando disponível. |
| `x-instance-name` | `Nome da instancia` | Nome publico da instancia. |
| `x-instance-id` | `1` | Identificador numerico interno da instancia. |
| `x-webhook-event` | `Nome externo do evento` | Mesmo valor do campo event no envelope. |

Exemplo de requisição recebida pelo consumidor:

```http
POST /webhooks/codechat HTTP/1.1
Host: example.com
Content-Type: application/json
User-Agent: CodeChat-Webhook/1.0
x-request-id: 019f0000-0000-7000-8000-000000000000
x-owner-jid: 5531999999999@s.whatsapp.net
x-instance-name: codechat
x-instance-id: 1
x-webhook-event: messages.upsert
```

`x-owner-jid` pode ser uma string vazia quando a instância ainda não estiver conectada ou quando o proprietário não estiver salvo no snapshot usado pelo dispatcher.

## Envelope padrão


```json
{
  "data": {},
  "event": "nome.do.evento",
  "instance": {
    "connectionStatus": "online",
    "externalAttributes": {},
    "id": 1,
    "name": "codechat",
    "ownerJid": "5531999999999@s.whatsapp.net"
  },
  "timestamp": "2026-07-04T18:00:00Z"
}
```

`event` é o nome externo do evento. `instance` contém o snapshot mínimo da instância responsável pelo evento. `data` contém os dados específicos de cada evento e pode ser objeto ou array. `timestamp` é gerado quando o envelope é criado, em RFC3339 UTC.

## Estrutura da instância


```json
{
  "connectionStatus": "online",
  "externalAttributes": {
    "tenantId": "019f0000-0000-7000-8000-000000000000"
  },
  "id": 1,
  "name": "codechat",
  "ownerJid": "5531999999999@s.whatsapp.net"
}
```

`id` é o identificador numérico interno da instância. `name` é o nome usado nas rotas. `connectionStatus` usa os valores persistidos da conexão, como `offline`, `connecting`, `qr_code`, `pairing_code`, `pairing`, `online`, `reconnecting`, `disconnected`, `connection_timeout`, `logged_out`, `session_missing`, `stream_replaced`, `keepalive_timeout`, `client_outdated`, `temporary_ban` e `connection_error`. `ownerJid` é `string` ou `null` no body; no header `x-owner-jid`, o valor nulo vira string vazia. `externalAttributes` sempre é um objeto JSON; valores ausentes, `null` ou inválidos são serializados como `{}`.

## Entrega e tratamento de erros


```json
{
  "delivery": {
    "allowedSchemes": [
      "http",
      "https"
    ],
    "concurrentSends": true,
    "contentType": "application/json",
    "method": "POST",
    "ordering": "not_guaranteed",
    "queueSize": 1000,
    "retryEnabled": false,
    "successStatus": "200-299",
    "timeoutSeconds": 15,
    "workers": 10
  }
}
```

- Somente respostas HTTP 2xx sao consideradas sucesso.
- Falhas de rede, timeout e respostas nao 2xx sao registradas em log e nao abortam o fluxo principal.
- Nao existe retry automatico nem dead-letter queue nesta implementacao.
- A fila de entrega e assincrona e processada por multiplos workers.
- A ordem relativa entre eventos nao e garantida entre instancias nem entre eventos diferentes da mesma instancia.
- Alguns eventos dependem de persistencia previa. Quando a persistencia falha, o evento pode nao ser emitido.
- Nao ha assinatura HMAC, header de autenticacao ou segredo compartilhado no emissor atual.
- Use HTTPS, endpoints privados, allowlist de IP ou autenticacao no destino quando houver dados sensiveis.
- O x-request-id serve para rastreio e correlacao; ele nao prova autenticidade e nao garante idempotencia.
- Se a fila estiver cheia, `Dispatch` retorna `ErrWebhookQueueFull`; os chamadores atuais registram aviso e continuam o fluxo principal.
- Durante shutdown, a fila é fechada e o processo aguarda os workers terminarem até o contexto de shutdown expirar.

## Eventos

### `call.upsert`

Atualizacao de chamada de voz ou video.

**Flag:** `callUpsert`

**Eventos internos:** `*events.CallOffer`, `*events.CallAccept`, `*events.CallOfferNotice`, `*events.CallPreAccept`, `*events.CallTransport`, `*events.CallTerminate`, `*events.CallReject`, `*events.CallRelayLatency`, `*events.UnknownCallEvent`

**Persistência:** Nao persiste dados especificos antes da entrega do webhook.

**Tipo de `data`:** `object`

**DTO/normalizador:** `CallUpsertWebhookData`

**Campos dinâmicos:** não

**Implementado em:** `internal/whatsapp/service.go`, `internal/whatsapp/webhook_extended_events.go`, `internal/webhook/payload.go`

#### Requisição

```http
POST /webhooks/codechat HTTP/1.1
Content-Type: application/json
x-webhook-event: call.upsert
```

#### Corpo

```json
{
  "data": {
    "callerPn": "5511999999999",
    "chatId": "5511999999999@s.whatsapp.net",
    "date": "2026-07-04T19:05:00Z",
    "from": "5511999999999@s.whatsapp.net",
    "groupJid": null,
    "id": "3EB0C4D0A1",
    "isGroup": false,
    "isVideo": false,
    "latencyMs": null,
    "offline": false,
    "status": "offer"
  },
  "event": "call.upsert",
  "instance": {
    "connectionStatus": "online",
    "externalAttributes": {},
    "id": 1,
    "name": "codechat",
    "ownerJid": "5511999999999@s.whatsapp.net"
  },
  "timestamp": "2026-07-04T18:00:00Z"
}
```

#### Campos de `data`

- `chatId`: `string`, obrigatório, não aceita `null`. JID do chat da chamada.
- `from`: `string`, obrigatório, não aceita `null`. JID de origem.
- `callerPn`: `string | null`, obrigatório, aceita `null`. Número de telefone do chamador quando disponível.
- `isGroup`: `boolean | null`, obrigatório, aceita `null`. Indica chamada em grupo quando o normalizador consegue inferir.
- `groupJid`: `string | null`, obrigatório, aceita `null`. JID do grupo quando disponivel.
- `id`: `string`, obrigatório, não aceita `null`. ID da chamada.
- `date`: `string`, obrigatório, não aceita `null`. Timestamp RFC3339 da chamada/processamento.
- `isVideo`: `boolean | null`, obrigatório, aceita `null`. Indica chamada de video quando o normalizador consegue inferir.
- `status`: `string`, obrigatório, não aceita `null`. Status normalizado da chamada. Valores possíveis: `offer`, `ringing`, `preaccept`, `transport`, `relaylatency`, `timeout`, `reject`, `accept`, `terminate`, `unknown`.
- `offline`: `boolean`, obrigatório, não aceita `null`. Indica se o evento veio como offline.
- `latencyMs`: `number | null`, obrigatório, aceita `null`. Latencia em milissegundos quando reportada.

#### Valores possíveis

- `status`: `offer`, `ringing`, `preaccept`, `transport`, `relaylatency`, `timeout`, `reject`, `accept`, `terminate`, `unknown`

#### Observações

- Sem observações adicionais.

### `chats.delete`

Exclusao ou limpeza de conversa.

**Flag:** `chatsDeleted`

**Eventos internos:** `*events.DeleteChat`

**Persistência:** Nao persiste dados especificos antes da entrega do webhook.

**Tipo de `data`:** `object`

**DTO/normalizador:** `ChatDeletedWebhookData`

**Campos dinâmicos:** sim

**Implementado em:** `internal/whatsapp/service.go`, `internal/whatsapp/webhook_events.go`

#### Requisição

```http
POST /webhooks/codechat HTTP/1.1
Content-Type: application/json
x-webhook-event: chats.delete
```

#### Corpo

```json
{
  "data": {
    "chatJid": "5511999999999@s.whatsapp.net",
    "dateTime": "2026-07-04T18:00:00Z",
    "deleteMedia": false
  },
  "event": "chats.delete",
  "instance": {
    "connectionStatus": "online",
    "externalAttributes": {},
    "id": 1,
    "name": "codechat",
    "ownerJid": "5511999999999@s.whatsapp.net"
  },
  "timestamp": "2026-07-04T18:00:00Z"
}
```

#### Campos de `data`

- `chatJid`: `string`, obrigatório, não aceita `null`. JID da conversa.
- `dateTime`: `string`, obrigatório, não aceita `null`. Timestamp RFC3339 do processamento.
- `deleteMedia`: `boolean`, opcional, não aceita `null`. Indica remocao de midia local quando presente.
- `additionalProperties`: `object`, opcional, não aceita `null`. Campos achatados da acao original.

#### Observações

- Sem observações adicionais.

### `chats.updated`

Atualizacao de propriedades de conversas.

**Flag:** `chatsUpdated`

**Eventos internos:** `*events.Blocklist`, `*events.BlocklistChange`, `*events.Archive`, `*events.UnarchiveChatsSetting`, `*events.ClearChat`, `*events.Pin`, `*events.Mute`, `*events.MarkChatAsRead`

**Persistência:** Nao persiste dados especificos antes da entrega do webhook.

**Tipo de `data`:** `object`

**DTO/normalizador:** `ChatUpdatedWebhookData`

**Campos dinâmicos:** sim

**Implementado em:** `internal/whatsapp/service.go`, `internal/whatsapp/webhook_events.go`

#### Requisição

```http
POST /webhooks/codechat HTTP/1.1
Content-Type: application/json
x-webhook-event: chats.updated
```

#### Corpo

```json
{
  "data": {
    "archived": true,
    "chatJid": "5511999999999@s.whatsapp.net",
    "dateTime": "2026-07-04T18:00:00Z",
    "type": "archive"
  },
  "event": "chats.updated",
  "instance": {
    "connectionStatus": "online",
    "externalAttributes": {},
    "id": 1,
    "name": "codechat",
    "ownerJid": "5511999999999@s.whatsapp.net"
  },
  "timestamp": "2026-07-04T18:00:00Z"
}
```

#### Campos de `data`

- `type`: `string`, obrigatório, não aceita `null`. Subtipo da atualizacao de chat. Valores possíveis: `blocklist`, `blocklist.change`, `archive`, `unarchive.setting`, `clear`, `pin`, `mute`, `mark.read`.
- `dateTime`: `string`, obrigatório, não aceita `null`. Timestamp RFC3339 UTC do evento ou do processamento.
- `chatJid`: `string`, opcional, não aceita `null`. JID da conversa quando o subtipo tem conversa especifica.
- `fromFullSync`: `boolean`, opcional, não aceita `null`. Indica se veio de sincronizacao completa quando disponivel.
- `additionalProperties`: `object`, opcional, não aceita `null`. Campos achatados do evento original do whatsmeow.

#### Valores possíveis

- `type`: `blocklist`, `blocklist.change`, `archive`, `unarchive.setting`, `clear`, `pin`, `mute`, `mark.read`

#### Observações

- Eventos UserStatusMute sao documentados em settings.update, porque o registro atual os roteia para esse evento externo.

### `connection.update`

Mudanca de estado da conexao da instancia.

**Flag:** `connectionUpdated`

**Eventos internos:** `*events.PairSuccess`, `*events.PairError`, `*events.Connected`, `*events.Disconnected`, `*events.LoggedOut`, `*events.StreamReplaced`, `*events.KeepAliveTimeout`, `*events.KeepAliveRestored`, `*events.ConnectFailure`, `*events.ManualLoginReconnect`, `*events.StreamError`, `*events.CATRefreshError`

**Persistência:** O status da instancia e atualizado pelos fluxos de conexao antes ou junto da entrega conforme o subtipo.

**Tipo de `data`:** `object`

**DTO/normalizador:** `ConnectionUpdateWebhookData`

**Campos dinâmicos:** não

**Implementado em:** `internal/whatsapp/service.go`, `internal/webhook/payload.go`

#### Requisição

```http
POST /webhooks/codechat HTTP/1.1
Content-Type: application/json
x-webhook-event: connection.update
```

#### Corpo

```json
{
  "data": {
    "connection": "open",
    "lastConnection": "2026-07-04T18:50:00Z",
    "type": "connected"
  },
  "event": "connection.update",
  "instance": {
    "connectionStatus": "online",
    "externalAttributes": {},
    "id": 1,
    "name": "codechat",
    "ownerJid": "5511999999999@s.whatsapp.net"
  },
  "timestamp": "2026-07-04T18:00:00Z"
}
```

#### Campos de `data`

- `type`: `string`, obrigatório, não aceita `null`. Subtipo normalizado da conexao. Valores possíveis: `pair.success`, `connected`, `disconnected`, `logged.out`, `stream.replaced`, `keepalive.timeout`, `keepalive.restored`, `connect.failure`, `manual.reconnect`, `pair.error`, `stream.error`, `cat.refresh.error`.
- `connection`: `string`, obrigatório, não aceita `null`. Estado externo da conexao. Valores possíveis: `connecting`, `open`, `close`, `replaced`, `timeout`.
- `statusReason`: `number`, opcional, não aceita `null`. Codigo numerico de motivo quando diferente de zero; omitido quando zero.
- `lastConnection`: `string`, opcional, não aceita `null`. Timestamp RFC3339 UTC quando informado; omitido quando ausente.
- `message`: `string`, opcional, não aceita `null`. Mensagem tecnica quando informada; omitida quando vazia.

#### Valores possíveis

- `type`: `pair.success`, `connected`, `disconnected`, `logged.out`, `stream.replaced`, `keepalive.timeout`, `keepalive.restored`, `connect.failure`, `manual.reconnect`, `pair.error`, `stream.error`, `cat.refresh.error`
- `connection`: `connecting`, `open`, `close`, `replaced`, `timeout`

#### Observações

- `statusReason`, `lastConnection` e `message` usam `omitempty`; quando estao zerados ou vazios, nao aparecem no JSON.

### `contacts.update`

Atualizacao parcial em contato existente.

**Flag:** `contactsUpdated`

**Eventos internos:** `*events.PushName`, `*events.BusinessName`

**Persistência:** Requer DATABASE_SAVE_DATA_CONTACTS=true. O contato e atualizado antes da entrega quando aplicavel.

**Flag de persistência:** `DATABASE_SAVE_DATA_CONTACTS`

**Tipo de `data`:** `array`

**DTO/normalizador:** `ContactUpdateWebhookData[]`

**Campos dinâmicos:** não

**Implementado em:** `internal/whatsapp/service.go`, `internal/whatsapp/event_persistence.go`, `internal/whatsapp/webhook_extended_events.go`

#### Requisição

```http
POST /webhooks/codechat HTTP/1.1
Content-Type: application/json
x-webhook-event: contacts.update
```

#### Corpo

```json
{
  "data": [
    {
      "action": "updated",
      "businessName": null,
      "id": 41,
      "lid": null,
      "pushName": "Cliente Atualizado",
      "remoteJid": "5511999999999@s.whatsapp.net",
      "source": "pushName"
    }
  ],
  "event": "contacts.update",
  "instance": {
    "connectionStatus": "online",
    "externalAttributes": {},
    "id": 1,
    "name": "codechat",
    "ownerJid": "5511999999999@s.whatsapp.net"
  },
  "timestamp": "2026-07-04T18:00:00Z"
}
```

#### Campos de `data`

- `id`: `number`, obrigatório, não aceita `null`. ID interno do contato persistido.
- `remoteJid`: `string`, obrigatório, não aceita `null`. JID remoto do contato.
- `lid`: `string | null`, obrigatório, aceita `null`. LID do contato quando conhecido.
- `pushName`: `string | null`, opcional, aceita `null`. Nome de exibição atualizado quando presente.
- `businessName`: `string | null`, opcional, aceita `null`. Nome comercial atualizado quando presente.
- `action`: `string`, obrigatório, não aceita `null`. Acao executada. Valores possíveis: `updated`.
- `source`: `string`, obrigatório, não aceita `null`. Origem da alteracao. Valores possíveis: `pushName`, `businessName`.

#### Valores possíveis

- `action`: `updated`
- `source`: `pushName`, `businessName`

#### Observações

- O payload e array; o handler atual normalmente envia um item por entrega.

### `contacts.upsert`

Contato criado ou atualizado no cadastro local.

**Flag:** `contactsUpsert`

**Eventos internos:** `*events.Contact`

**Persistência:** Requer DATABASE_SAVE_DATA_CONTACTS=true. O contato e salvo antes da entrega.

**Flag de persistência:** `DATABASE_SAVE_DATA_CONTACTS`

**Tipo de `data`:** `object`

**DTO/normalizador:** `ContactUpsertWebhookData`

**Campos dinâmicos:** não

**Implementado em:** `internal/whatsapp/service.go`, `internal/whatsapp/event_persistence.go`

#### Requisição

```http
POST /webhooks/codechat HTTP/1.1
Content-Type: application/json
x-webhook-event: contacts.upsert
```

#### Corpo

```json
{
  "data": {
    "action": "upserted",
    "id": 41,
    "lid": "279847268053216@lid",
    "profilePicUrl": null,
    "pushName": "Cliente",
    "remoteJid": "5511999999999@s.whatsapp.net"
  },
  "event": "contacts.upsert",
  "instance": {
    "connectionStatus": "online",
    "externalAttributes": {},
    "id": 1,
    "name": "codechat",
    "ownerJid": "5511999999999@s.whatsapp.net"
  },
  "timestamp": "2026-07-04T18:00:00Z"
}
```

#### Campos de `data`

- `id`: `number`, obrigatório, não aceita `null`. ID interno do contato persistido.
- `remoteJid`: `string`, obrigatório, não aceita `null`. JID remoto do contato.
- `lid`: `string | null`, obrigatório, aceita `null`. LID do contato quando conhecido.
- `pushName`: `string | null`, obrigatório, aceita `null`. Nome de exibição salvo para o contato.
- `profilePicUrl`: `string | null`, obrigatório, aceita `null`. URL de foto do perfil quando conhecida.
- `action`: `string`, obrigatório, não aceita `null`. Acao executada. Valores possíveis: `upserted`.

#### Valores possíveis

- `action`: `upserted`

#### Observações

- Sem observações adicionais.

### `groups.participants.update`

Mudanca de participantes em grupo.

**Flag:** `groupsParticipantsUpdated`

**Eventos internos:** `*events.GroupInfo`

**Persistência:** Nao persiste dados especificos antes da entrega do webhook.

**Tipo de `data`:** `object`

**DTO/normalizador:** `GroupParticipantsUpdatedWebhookData`

**Campos dinâmicos:** não

**Implementado em:** `internal/whatsapp/service.go`, `internal/whatsapp/webhook_extended_events.go`

#### Requisição

```http
POST /webhooks/codechat HTTP/1.1
Content-Type: application/json
x-webhook-event: groups.participants.update
```

#### Corpo

```json
{
  "data": {
    "action": "add",
    "author": "5531999999999@s.whatsapp.net",
    "authorPn": "5531999999999",
    "id": "120363000000000000@g.us",
    "participants": [
      {
        "admin": null,
        "id": "5511999999999@s.whatsapp.net",
        "isAdmin": false,
        "isSuperAdmin": false
      }
    ]
  },
  "event": "groups.participants.update",
  "instance": {
    "connectionStatus": "online",
    "externalAttributes": {},
    "id": 1,
    "name": "codechat",
    "ownerJid": "5511999999999@s.whatsapp.net"
  },
  "timestamp": "2026-07-04T18:00:00Z"
}
```

#### Campos de `data`

- `id`: `string`, obrigatório, não aceita `null`. JID do grupo.
- `author`: `string`, obrigatório, não aceita `null`. JID do autor da alteracao; string vazia quando ausente.
- `authorPn`: `string`, opcional, não aceita `null`. Número de telefone do autor quando disponível; omitido quando ausente.
- `participants`: `GroupParticipantWebhookData[]`, obrigatório, não aceita `null`. Participantes afetados.
- `action`: `string`, obrigatório, não aceita `null`. Acao aplicada. Valores possíveis: `add`, `remove`, `promote`, `demote`.

#### Valores possíveis

- `action`: `add`, `remove`, `promote`, `demote`

#### Observações

- Sem observações adicionais.

### `groups.update`

Atualizacao parcial de metadados de grupo.

**Flag:** `groupsUpdated`

**Eventos internos:** `*events.GroupInfo`

**Persistência:** Nao persiste dados especificos antes da entrega do webhook.

**Tipo de `data`:** `array`

**DTO/normalizador:** `GroupUpdateWebhookData[]`

**Campos dinâmicos:** não

**Implementado em:** `internal/whatsapp/service.go`, `internal/whatsapp/webhook_extended_events.go`

#### Requisição

```http
POST /webhooks/codechat HTTP/1.1
Content-Type: application/json
x-webhook-event: groups.update
```

#### Corpo

```json
{
  "data": [
    {
      "partial": {
        "announce": true,
        "id": "120363000000000000@g.us",
        "subject": "Novo nome do grupo",
        "subjectTime": 1783188000
      }
    }
  ],
  "event": "groups.update",
  "instance": {
    "connectionStatus": "online",
    "externalAttributes": {},
    "id": 1,
    "name": "codechat",
    "ownerJid": "5511999999999@s.whatsapp.net"
  },
  "timestamp": "2026-07-04T18:00:00Z"
}
```

#### Campos de `data`

- `partial`: `GroupPartialWebhookData`, obrigatório, não aceita `null`. Metadados parciais alterados no grupo.
- `partial.notify`: `string`, opcional, não aceita `null`. Nome de notificacao do grupo quando informado.
- `partial.addressingMode`: `string`, opcional, não aceita `null`. Modo de enderecamento do grupo quando informado.
- `partial.owner`: `string`, opcional, não aceita `null`. JID do owner quando informado.
- `partial.ownerPn`: `string`, opcional, não aceita `null`. Número de telefone do proprietário quando informado.
- `partial.ownerUsername`: `string`, opcional, não aceita `null`. Nome de usuário do proprietário quando informado.
- `partial.ownerCountryCode`: `string`, opcional, não aceita `null`. Codigo de pais do owner quando informado.
- `partial.subjectOwner`: `string`, opcional, não aceita `null`. JID de quem definiu o subject quando informado.
- `partial.subjectOwnerPn`: `string`, opcional, não aceita `null`. Número de telefone de quem definiu o subject quando informado.
- `partial.subjectOwnerUsername`: `string`, opcional, não aceita `null`. Nome de usuário de quem definiu o subject quando informado.
- `partial.subjectTime`: `number`, opcional, não aceita `null`. Timestamp Unix do subject quando informado.
- `partial.creation`: `number`, opcional, não aceita `null`. Timestamp Unix de criacao quando informado.
- `partial.desc`: `string`, opcional, não aceita `null`. Descricao do grupo quando informada.
- `partial.descOwner`: `string`, opcional, não aceita `null`. JID de quem definiu a descricao quando informado.
- `partial.descOwnerPn`: `string`, opcional, não aceita `null`. Número de telefone de quem definiu a descricao quando informado.
- `partial.descOwnerUsername`: `string`, opcional, não aceita `null`. Nome de usuário de quem definiu a descricao quando informado.
- `partial.descId`: `string`, opcional, não aceita `null`. ID da descricao quando informado.
- `partial.descTime`: `number`, opcional, não aceita `null`. Timestamp Unix da descricao quando informado.
- `partial.linkedParent`: `string`, opcional, não aceita `null`. Grupo/comunidade pai quando informado.
- `partial.restrict`: `boolean`, opcional, não aceita `null`. Restricao de edicao quando informada.
- `partial.announce`: `boolean`, opcional, não aceita `null`. Modo anuncio quando informado.
- `partial.memberAddMode`: `boolean`, opcional, não aceita `null`. Modo de adicao por membros quando informado.
- `partial.joinApprovalMode`: `boolean`, opcional, não aceita `null`. Modo de aprovacao de entrada quando informado.
- `partial.isCommunity`: `boolean`, opcional, não aceita `null`. Indica comunidade quando informado.
- `partial.isCommunityAnnounce`: `boolean`, opcional, não aceita `null`. Indica grupo de anuncios da comunidade quando informado.
- `partial.size`: `number`, opcional, não aceita `null`. Tamanho do grupo quando informado.
- `partial.ephemeralDuration`: `number`, opcional, não aceita `null`. Duracao de mensagens temporarias em segundos quando informada.
- `partial.inviteCode`: `string`, opcional, não aceita `null`. Codigo de convite quando informado.
- `partial.author`: `string`, opcional, não aceita `null`. Autor da alteracao quando informado.
- `partial.authorPn`: `string`, opcional, não aceita `null`. Número de telefone do autor quando informado.
- `partial.authorUsername`: `string`, opcional, não aceita `null`. Nome de usuário do autor quando informado.

#### Observações

- O handler atual envia array com um item contendo partial.

### `groups.upsert`

Grupo criado, descoberto ou sincronizado.

**Flag:** `groupsUpsert`

**Eventos internos:** `*events.JoinedGroup`

**Persistência:** Nao persiste dados especificos antes da entrega do webhook.

**Tipo de `data`:** `array`

**DTO/normalizador:** `GroupUpsertWebhookData[]`

**Campos dinâmicos:** não

**Implementado em:** `internal/whatsapp/service.go`, `internal/whatsapp/webhook_extended_events.go`

#### Requisição

```http
POST /webhooks/codechat HTTP/1.1
Content-Type: application/json
x-webhook-event: groups.upsert
```

#### Corpo

```json
{
  "data": [
    {
      "addressingMode": "pn",
      "creation": 1783187000,
      "id": "120363000000000000@g.us",
      "isCommunity": false,
      "owner": "5531999999999@s.whatsapp.net",
      "participants": [
        {
          "admin": "admin",
          "id": "5511999999999@s.whatsapp.net",
          "isAdmin": true,
          "isSuperAdmin": false,
          "lid": "279847268053216@lid"
        }
      ],
      "subject": "Grupo"
    }
  ],
  "event": "groups.upsert",
  "instance": {
    "connectionStatus": "online",
    "externalAttributes": {},
    "id": 1,
    "name": "codechat",
    "ownerJid": "5511999999999@s.whatsapp.net"
  },
  "timestamp": "2026-07-04T18:00:00Z"
}
```

#### Campos de `data`

- `id`: `string`, obrigatório, não aceita `null`. JID do grupo.
- `subject`: `string`, obrigatório, não aceita `null`. Nome do grupo.
- `participants`: `GroupParticipantWebhookData[]`, obrigatório, não aceita `null`. Participantes conhecidos do grupo.
- `notify`: `string`, opcional, não aceita `null`. Nome de notificacao do grupo quando informado.
- `addressingMode`: `string`, opcional, não aceita `null`. Modo de enderecamento do grupo quando informado.
- `owner`: `string`, opcional, não aceita `null`. JID do owner quando informado.
- `ownerPn`: `string`, opcional, não aceita `null`. Número de telefone do proprietário quando informado.
- `ownerUsername`: `string`, opcional, não aceita `null`. Nome de usuário do proprietário quando informado.
- `ownerCountryCode`: `string`, opcional, não aceita `null`. Codigo de pais do owner quando informado.
- `subjectOwner`: `string`, opcional, não aceita `null`. JID de quem definiu o subject quando informado.
- `subjectOwnerPn`: `string`, opcional, não aceita `null`. Número de telefone de quem definiu o subject quando informado.
- `subjectOwnerUsername`: `string`, opcional, não aceita `null`. Nome de usuário de quem definiu o subject quando informado.
- `subjectTime`: `number`, opcional, não aceita `null`. Timestamp Unix do subject quando informado.
- `creation`: `number`, opcional, não aceita `null`. Timestamp Unix de criacao quando informado.
- `desc`: `string`, opcional, não aceita `null`. Descricao do grupo quando informada.
- `descOwner`: `string`, opcional, não aceita `null`. JID de quem definiu a descricao quando informado.
- `descOwnerPn`: `string`, opcional, não aceita `null`. Número de telefone de quem definiu a descricao quando informado.
- `descOwnerUsername`: `string`, opcional, não aceita `null`. Nome de usuário de quem definiu a descricao quando informado.
- `descId`: `string`, opcional, não aceita `null`. ID da descricao quando informado.
- `descTime`: `number`, opcional, não aceita `null`. Timestamp Unix da descricao quando informado.
- `linkedParent`: `string`, opcional, não aceita `null`. Grupo/comunidade pai quando informado.
- `restrict`: `boolean`, opcional, não aceita `null`. Restricao de edicao quando informada.
- `announce`: `boolean`, opcional, não aceita `null`. Modo anuncio quando informado.
- `memberAddMode`: `boolean`, opcional, não aceita `null`. Modo de adicao por membros quando informado.
- `joinApprovalMode`: `boolean`, opcional, não aceita `null`. Modo de aprovacao de entrada quando informado.
- `isCommunity`: `boolean`, opcional, não aceita `null`. Indica comunidade quando informado.
- `isCommunityAnnounce`: `boolean`, opcional, não aceita `null`. Indica grupo de anuncios da comunidade quando informado.
- `size`: `number`, opcional, não aceita `null`. Tamanho do grupo quando informado.
- `ephemeralDuration`: `number`, opcional, não aceita `null`. Duracao de mensagens temporarias em segundos quando informada.
- `inviteCode`: `string`, opcional, não aceita `null`. Codigo de convite quando informado.
- `author`: `string`, opcional, não aceita `null`. Autor da alteracao quando informado.
- `authorPn`: `string`, opcional, não aceita `null`. Número de telefone do autor quando informado.
- `authorUsername`: `string`, opcional, não aceita `null`. Nome de usuário do autor quando informado.

#### Observações

- O payload e array para compatibilidade com contratos de lista, mesmo quando uma entrega contem um grupo.

### `history.sync`

Sincronizacao de historico recebida do WhatsApp.

**Flag:** `historySync`

**Eventos internos:** `*events.HistorySync`

**Persistência:** Nao persiste dados especificos antes da entrega do webhook.

**Tipo de `data`:** `object`

**DTO/normalizador:** `HistorySyncWebhookData`

**Campos dinâmicos:** sim

**Implementado em:** `internal/whatsapp/service.go`, `internal/whatsapp/webhook_extended_events.go`

#### Requisição

```http
POST /webhooks/codechat HTTP/1.1
Content-Type: application/json
x-webhook-event: history.sync
```

#### Corpo

```json
{
  "data": {
    "data": {
      "syncType": "INITIAL_BOOTSTRAP"
    },
    "dateTime": "2026-07-04T18:00:00Z",
    "type": "history.sync"
  },
  "event": "history.sync",
  "instance": {
    "connectionStatus": "online",
    "externalAttributes": {},
    "id": 1,
    "name": "codechat",
    "ownerJid": "5511999999999@s.whatsapp.net"
  },
  "timestamp": "2026-07-04T18:00:00Z"
}
```

#### Campos de `data`

- `type`: `string`, obrigatório, não aceita `null`. Tipo fixo do payload. Valores possíveis: `history.sync`.
- `dateTime`: `string`, obrigatório, não aceita `null`. Timestamp RFC3339 UTC do evento ou do processamento.
- `data`: `object`, opcional, não aceita `null`. Conteudo normalizado do evento de historico quando disponivel.

#### Valores possíveis

- `type`: `history.sync`

#### Observações

- Payload dinamico porque o conteudo vem do proto de historico do whatsmeow.

### `identity.update`

Mudanca de identidade criptografica de um contato.

**Flag:** `identityUpdated`

**Eventos internos:** `*events.IdentityChange`

**Persistência:** Nao persiste dados especificos antes da entrega do webhook.

**Tipo de `data`:** `object`

**DTO/normalizador:** `IdentityUpdatedWebhookData`

**Campos dinâmicos:** não

**Implementado em:** `internal/whatsapp/service.go`, `internal/webhook/payload.go`

#### Requisição

```http
POST /webhooks/codechat HTTP/1.1
Content-Type: application/json
x-webhook-event: identity.update
```

#### Corpo

```json
{
  "data": {
    "dateTime": "2026-07-04T18:00:00Z",
    "implicit": true,
    "jid": "5511999999999@s.whatsapp.net"
  },
  "event": "identity.update",
  "instance": {
    "connectionStatus": "online",
    "externalAttributes": {},
    "id": 1,
    "name": "codechat",
    "ownerJid": "5511999999999@s.whatsapp.net"
  },
  "timestamp": "2026-07-04T18:00:00Z"
}
```

#### Campos de `data`

- `jid`: `string`, obrigatório, não aceita `null`. JID cuja identidade mudou.
- `dateTime`: `string`, obrigatório, não aceita `null`. Timestamp RFC3339 UTC do evento ou do processamento.
- `implicit`: `boolean`, obrigatório, não aceita `null`. Indica mudanca implicita reportada pelo whatsmeow.

#### Observações

- Sem observações adicionais.

### `labels.association`

Associacao ou remocao de label em chat ou mensagem.

**Flag:** `labelsAssociation`

**Eventos internos:** `*events.LabelAssociationChat`, `*events.LabelAssociationMessage`

**Persistência:** Nao persiste dados especificos antes da entrega do webhook.

**Tipo de `data`:** `object`

**DTO/normalizador:** `LabelsAssociationWebhookData`

**Campos dinâmicos:** sim

**Implementado em:** `internal/whatsapp/service.go`, `internal/webhook/payload.go`

#### Requisição

```http
POST /webhooks/codechat HTTP/1.1
Content-Type: application/json
x-webhook-event: labels.association
```

#### Corpo

```json
{
  "data": {
    "action": "add",
    "chatJid": "5511999999999@s.whatsapp.net",
    "dateTime": "2026-07-04T18:00:00Z",
    "labelId": "7",
    "type": "chat"
  },
  "event": "labels.association",
  "instance": {
    "connectionStatus": "online",
    "externalAttributes": {},
    "id": 1,
    "name": "codechat",
    "ownerJid": "5511999999999@s.whatsapp.net"
  },
  "timestamp": "2026-07-04T18:00:00Z"
}
```

#### Campos de `data`

- `type`: `string`, obrigatório, não aceita `null`. Tipo de associacao. Valores possíveis: `chat`, `message`.
- `chatJid`: `string`, obrigatório, não aceita `null`. JID da conversa.
- `messageId`: `string`, opcional, não aceita `null`. ID da mensagem quando type=message.
- `labelId`: `string`, obrigatório, não aceita `null`. ID da label.
- `action`: `string`, opcional, não aceita `null`. Acao inferida quando labeled esta presente. Valores possíveis: `add`, `remove`.
- `dateTime`: `string`, obrigatório, não aceita `null`. Timestamp RFC3339 do processamento.
- `additionalProperties`: `object`, opcional, não aceita `null`. Campos achatados do evento original.

#### Valores possíveis

- `type`: `chat`, `message`
- `action`: `add`, `remove`

#### Observações

- Sem observações adicionais.

### `labels.edit`

Criacao, alteracao ou remocao de label.

**Flag:** `labelsEdit`

**Eventos internos:** `*events.LabelEdit`

**Persistência:** Nao persiste dados especificos antes da entrega do webhook.

**Tipo de `data`:** `object`

**DTO/normalizador:** `LabelsEditWebhookData`

**Campos dinâmicos:** sim

**Implementado em:** `internal/whatsapp/service.go`, `internal/whatsapp/webhook_extended_events.go`

#### Requisição

```http
POST /webhooks/codechat HTTP/1.1
Content-Type: application/json
x-webhook-event: labels.edit
```

#### Corpo

```json
{
  "data": {
    "color": 3,
    "deleted": false,
    "id": "12",
    "name": "Cliente"
  },
  "event": "labels.edit",
  "instance": {
    "connectionStatus": "online",
    "externalAttributes": {},
    "id": 1,
    "name": "codechat",
    "ownerJid": "5511999999999@s.whatsapp.net"
  },
  "timestamp": "2026-07-04T18:00:00Z"
}
```

#### Campos de `data`

- `id`: `string`, obrigatório, não aceita `null`. ID da label, derivado de labelId.
- `name`: `string`, opcional, não aceita `null`. Nome da label quando informado.
- `color`: `number`, opcional, não aceita `null`. Cor da label quando informada.
- `deleted`: `boolean`, opcional, não aceita `null`. Indica label removida quando informado.
- `additionalProperties`: `object`, opcional, não aceita `null`. Campos achatados do evento original.

#### Observações

- O normalizador nao adiciona campo `type` nem `dateTime` para este evento.

### `media.retry`

Resultado ou erro relacionado a tentativa de retry de midia.

**Flag:** `mediaRetry`

**Eventos internos:** `*events.MediaRetry`

**Persistência:** Nao persiste dados especificos antes da entrega do webhook.

**Tipo de `data`:** `object`

**DTO/normalizador:** `MediaRetryWebhookData`

**Campos dinâmicos:** não

**Implementado em:** `internal/whatsapp/service.go`, `internal/whatsapp/webhook_events.go`, `internal/webhook/payload.go`

#### Requisição

```http
POST /webhooks/codechat HTTP/1.1
Content-Type: application/json
x-webhook-event: media.retry
```

#### Corpo

```json
{
  "data": {
    "chatJid": "5511999999999@s.whatsapp.net",
    "dateTime": "2026-07-04T18:00:00Z",
    "errorCode": 404,
    "hasCiphertext": true,
    "keyFromMe": false,
    "messageId": "ABC123",
    "senderJid": "5511988888888@s.whatsapp.net"
  },
  "event": "media.retry",
  "instance": {
    "connectionStatus": "online",
    "externalAttributes": {},
    "id": 1,
    "name": "codechat",
    "ownerJid": "5511999999999@s.whatsapp.net"
  },
  "timestamp": "2026-07-04T18:00:00Z"
}
```

#### Campos de `data`

- `messageId`: `string`, obrigatório, não aceita `null`. ID da mensagem.
- `chatJid`: `string`, obrigatório, não aceita `null`. JID da conversa.
- `senderJid`: `string`, opcional, não aceita `null`. JID do remetente quando disponivel; omitido quando ausente.
- `keyFromMe`: `boolean`, obrigatório, não aceita `null`. Indica se a mensagem e da propria instancia.
- `hasCiphertext`: `boolean`, obrigatório, não aceita `null`. Indica se o evento carregou ciphertext.
- `errorCode`: `number`, opcional, não aceita `null`. Codigo de erro quando informado; omitido quando ausente.
- `dateTime`: `string`, obrigatório, não aceita `null`. Timestamp RFC3339 UTC do evento ou do processamento.

#### Observações

- Ciphertext e IV recebidos pelo whatsmeow nao sao expostos no webhook.

### `messages.delete`

Mensagem removida localmente pelo evento DeleteForMe.

**Flag:** `messagesDeleted`

**Eventos internos:** `*events.DeleteForMe`

**Persistência:** Nao persiste dados especificos antes da entrega do webhook.

**Tipo de `data`:** `object`

**DTO/normalizador:** `MessageDeletedWebhookData`

**Campos dinâmicos:** não

**Implementado em:** `internal/whatsapp/service.go`, `internal/whatsapp/webhook_events.go`

#### Requisição

```http
POST /webhooks/codechat HTTP/1.1
Content-Type: application/json
x-webhook-event: messages.delete
```

#### Corpo

```json
{
  "data": {
    "chatJid": "5511999999999@s.whatsapp.net",
    "dateTime": "2026-07-04T18:00:00Z",
    "deleteMedia": true,
    "fromFullSync": false,
    "keyFromMe": false,
    "messageId": "ABC123",
    "messageTime": "2026-07-04T17:59:00Z",
    "senderJid": "5511988888888@s.whatsapp.net"
  },
  "event": "messages.delete",
  "instance": {
    "connectionStatus": "online",
    "externalAttributes": {},
    "id": 1,
    "name": "codechat",
    "ownerJid": "5511999999999@s.whatsapp.net"
  },
  "timestamp": "2026-07-04T18:00:00Z"
}
```

#### Campos de `data`

- `chatJid`: `string`, obrigatório, não aceita `null`. JID da conversa.
- `senderJid`: `string`, opcional, não aceita `null`. JID do remetente quando disponivel; omitido quando ausente.
- `keyFromMe`: `boolean`, obrigatório, não aceita `null`. Indica se a mensagem era da propria instancia.
- `messageId`: `string`, obrigatório, não aceita `null`. ID da mensagem apagada.
- `deleteMedia`: `boolean`, obrigatório, não aceita `null`. Indica se a midia local deve ser removida.
- `fromFullSync`: `boolean`, obrigatório, não aceita `null`. Indica se veio de sincronizacao completa.
- `dateTime`: `string`, obrigatório, não aceita `null`. Timestamp RFC3339 UTC do evento ou do processamento.
- `messageTime`: `string`, opcional, não aceita `null`. Timestamp RFC3339 UTC original da mensagem quando informado; omitido quando ausente.

#### Observações

- Sem observações adicionais.

### `messages.star`

Marcacao ou desmarcacao de estrela em mensagem.

**Flag:** `messagesStarred`

**Eventos internos:** `*events.Star`

**Persistência:** Nao persiste dados especificos antes da entrega do webhook.

**Tipo de `data`:** `object`

**DTO/normalizador:** `MessageStarredWebhookData`

**Campos dinâmicos:** não

**Implementado em:** `internal/whatsapp/service.go`, `internal/whatsapp/webhook_events.go`

#### Requisição

```http
POST /webhooks/codechat HTTP/1.1
Content-Type: application/json
x-webhook-event: messages.star
```

#### Corpo

```json
{
  "data": {
    "chatJid": "5511999999999@s.whatsapp.net",
    "dateTime": "2026-07-04T18:00:00Z",
    "fromFullSync": false,
    "keyFromMe": false,
    "messageId": "ABC123",
    "senderJid": "5511988888888@s.whatsapp.net",
    "starred": true
  },
  "event": "messages.star",
  "instance": {
    "connectionStatus": "online",
    "externalAttributes": {},
    "id": 1,
    "name": "codechat",
    "ownerJid": "5511999999999@s.whatsapp.net"
  },
  "timestamp": "2026-07-04T18:00:00Z"
}
```

#### Campos de `data`

- `chatJid`: `string`, obrigatório, não aceita `null`. JID da conversa.
- `senderJid`: `string`, opcional, não aceita `null`. JID do remetente quando disponivel; omitido quando ausente.
- `keyFromMe`: `boolean`, obrigatório, não aceita `null`. Indica se a mensagem e da propria instancia.
- `messageId`: `string`, obrigatório, não aceita `null`. ID da mensagem.
- `starred`: `boolean`, obrigatório, não aceita `null`. true quando marcada com estrela.
- `fromFullSync`: `boolean`, obrigatório, não aceita `null`. Indica se veio de sincronizacao completa.
- `dateTime`: `string`, obrigatório, não aceita `null`. Timestamp RFC3339 UTC do evento ou do processamento.

#### Observações

- Sem observações adicionais.

### `messages.undecryptable`

Mensagem recebida sem possibilidade de descriptografia.

**Flag:** `messagesUndecryptable`

**Eventos internos:** `*events.UndecryptableMessage`

**Persistência:** Nao persiste dados especificos antes da entrega do webhook.

**Tipo de `data`:** `object`

**DTO/normalizador:** `MessageUndecryptableWebhookData`

**Campos dinâmicos:** não

**Implementado em:** `internal/whatsapp/service.go`, `internal/whatsapp/webhook_events.go`

#### Requisição

```http
POST /webhooks/codechat HTTP/1.1
Content-Type: application/json
x-webhook-event: messages.undecryptable
```

#### Corpo

```json
{
  "data": {
    "chatJid": "5511999999999@s.whatsapp.net",
    "dateTime": "2026-07-04T18:00:00Z",
    "decryptFailMode": "hide",
    "isUnavailable": true,
    "keyFromMe": false,
    "keyId": "ABC123",
    "senderJid": "5511988888888@s.whatsapp.net",
    "unavailableType": "view_once"
  },
  "event": "messages.undecryptable",
  "instance": {
    "connectionStatus": "online",
    "externalAttributes": {},
    "id": 1,
    "name": "codechat",
    "ownerJid": "5511999999999@s.whatsapp.net"
  },
  "timestamp": "2026-07-04T18:00:00Z"
}
```

#### Campos de `data`

- `keyId`: `string`, obrigatório, não aceita `null`. ID da chave/mensagem que falhou.
- `chatJid`: `string`, obrigatório, não aceita `null`. JID da conversa.
- `senderJid`: `string`, opcional, não aceita `null`. JID do remetente quando disponivel; omitido quando ausente.
- `keyFromMe`: `boolean`, obrigatório, não aceita `null`. Indica se a mensagem e da propria instancia.
- `isUnavailable`: `boolean`, obrigatório, não aceita `null`. Indica se o conteudo foi marcado como indisponivel.
- `unavailableType`: `string`, opcional, não aceita `null`. Tipo de indisponibilidade. Valores possíveis: `view_once`.
- `decryptFailMode`: `string`, opcional, não aceita `null`. Modo de falha reportado. Valores possíveis: `hide`.
- `dateTime`: `string`, obrigatório, não aceita `null`. Timestamp RFC3339 UTC do evento ou do processamento.

#### Valores possíveis

- `unavailableType`: `view_once`
- `decryptFailMode`: `hide`

#### Observações

- Campos vazios sao omitidos por omitempty.

### `messages.update`

Atualizacao de recibo/status de uma mensagem ja conhecida.

**Flag:** `messagesUpdated`

**Eventos internos:** `*events.Receipt`

**Persistência:** Requer DATABASE_SAVE_MESSAGE_UPDATE=true. O handler tenta localizar a mensagem, persiste a atualizacao e so entao entrega.

**Flag de persistência:** `DATABASE_SAVE_MESSAGE_UPDATE`

**Tipo de `data`:** `object`

**DTO/normalizador:** `MessageUpdateWebhookData`

**Campos dinâmicos:** não

**Implementado em:** `internal/whatsapp/service.go`, `internal/whatsapp/event_persistence.go`, `internal/webhook/payload.go`

#### Requisição

```http
POST /webhooks/codechat HTTP/1.1
Content-Type: application/json
x-webhook-event: messages.update
```

#### Corpo

```json
{
  "data": {
    "dateTime": "2026-07-04T18:05:00Z",
    "messageId": 1024,
    "status": "read"
  },
  "event": "messages.update",
  "instance": {
    "connectionStatus": "online",
    "externalAttributes": {},
    "id": 1,
    "name": "codechat",
    "ownerJid": "5511999999999@s.whatsapp.net"
  },
  "timestamp": "2026-07-04T18:00:00Z"
}
```

#### Campos de `data`

- `messageId`: `number`, obrigatório, não aceita `null`. ID interno da mensagem persistida.
- `status`: `string`, obrigatório, não aceita `null`. Status normalizado do recibo. Valores possíveis: `delivered`, `sent`, `read`, `played`, `server_error`, `retry`, `unknown`.
- `dateTime`: `string`, obrigatório, não aceita `null`. Timestamp RFC3339 UTC do recibo; usa o timestamp do evento ou o horario do processamento.

#### Valores possíveis

- `status`: `delivered`, `sent`, `read`, `played`, `server_error`, `retry`, `unknown`

#### Observações

- Quando a mensagem referenciada nao e encontrada, o evento e descartado apos as tentativas configuradas no handler.

### `messages.upsert`

Mensagem recebida e persistida pela aplicacao.

**Flag:** `messagesUpsert`

**Eventos internos:** `*events.Message`, `*events.FBMessage`

**Persistência:** Requer DATABASE_SAVE_DATA_NEW_MESSAGE=true. A mensagem e persistida com CreateOrIgnore e relida antes da entrega.

**Flag de persistência:** `DATABASE_SAVE_DATA_NEW_MESSAGE`

**Tipo de `data`:** `object`

**DTO/normalizador:** `MessageWebhookData`

**Campos dinâmicos:** não

**Implementado em:** `internal/whatsapp/service.go`, `internal/whatsapp/event_persistence.go`, `internal/webhook/payload.go`

#### Requisição

```http
POST /webhooks/codechat HTTP/1.1
Content-Type: application/json
x-webhook-event: messages.upsert
```

#### Corpo

```json
{
  "data": {
    "content": {
      "text": "Ola"
    },
    "device": "ios",
    "id": 1024,
    "isGroup": false,
    "keyFromMe": false,
    "keyLid": null,
    "keyParticipant": null,
    "keyParticipantLid": null,
    "keyRemoteJid": "5511999999999@s.whatsapp.net",
    "messageTimestamp": 1783188000,
    "messageType": "conversation",
    "metadata": null,
    "pushName": "Cliente"
  },
  "event": "messages.upsert",
  "instance": {
    "connectionStatus": "online",
    "externalAttributes": {},
    "id": 1,
    "name": "codechat",
    "ownerJid": "5511999999999@s.whatsapp.net"
  },
  "timestamp": "2026-07-04T18:00:00Z"
}
```

#### Campos de `data`

- `id`: `number`, obrigatório, não aceita `null`. ID interno da mensagem.
- `keyRemoteJid`: `string | null`, obrigatório, aceita `null`. JID remoto da mensagem.
- `keyLid`: `string | null`, obrigatório, aceita `null`. LID remoto da mensagem.
- `keyFromMe`: `boolean`, obrigatório, não aceita `null`. Indica se a mensagem foi enviada pela propria instancia.
- `keyParticipant`: `string | null`, obrigatório, aceita `null`. Participante em mensagens de grupo.
- `keyParticipantLid`: `string | null`, obrigatório, aceita `null`. LID do participante em mensagens de grupo.
- `pushName`: `string | null`, obrigatório, aceita `null`. Nome exibido do remetente quando conhecido.
- `messageType`: `string`, obrigatório, não aceita `null`. Tipo normalizado da mensagem.
- `content`: `object`, obrigatório, não aceita `null`. Conteudo normalizado da mensagem.
- `messageTimestamp`: `number`, obrigatório, não aceita `null`. Timestamp Unix em segundos.
- `device`: `string | null`, obrigatório, aceita `null`. Dispositivo/origem inferida da mensagem.
- `isGroup`: `boolean`, obrigatório, não aceita `null`. Indica se a mensagem pertence a grupo.
- `metadata`: `object | null`, obrigatório, aceita `null`. Metadados adicionais normalizados.

#### Observações

- Se a persistencia ou a releitura da mensagem falhar, o webhook nao e emitido.

### `news.letter`

Eventos relacionados a newsletters/canais.

**Flag:** `newsLetter`

**Eventos internos:** `*events.NewsletterJoin`, `*events.NewsletterLeave`, `*events.NewsletterLiveUpdate`, `*events.NewsletterMessageMeta`, `*events.NewsletterMuteChange`

**Persistência:** Nao persiste dados especificos antes da entrega do webhook.

**Tipo de `data`:** `object`

**DTO/normalizador:** `NewsLetterWebhookData`

**Campos dinâmicos:** sim

**Implementado em:** `internal/whatsapp/service.go`, `internal/whatsapp/webhook_extended_events.go`

#### Requisição

```http
POST /webhooks/codechat HTTP/1.1
Content-Type: application/json
x-webhook-event: news.letter
```

#### Corpo

```json
{
  "data": {
    "dateTime": "2026-07-04T18:00:00Z",
    "muted": true,
    "newsletterJid": "120363000000000000@newsletter",
    "type": "mute.change"
  },
  "event": "news.letter",
  "instance": {
    "connectionStatus": "online",
    "externalAttributes": {},
    "id": 1,
    "name": "codechat",
    "ownerJid": "5511999999999@s.whatsapp.net"
  },
  "timestamp": "2026-07-04T18:00:00Z"
}
```

#### Campos de `data`

- `type`: `string`, obrigatório, não aceita `null`. Subtipo do evento de newsletter. Valores possíveis: `join`, `leave`, `live.update`, `message.meta`, `mute.change`.
- `newsletterJid`: `string`, opcional, não aceita `null`. JID da newsletter quando a origem traz id ou jid.
- `dateTime`: `string`, obrigatório, não aceita `null`. Timestamp RFC3339 do processamento.
- `additionalProperties`: `object`, opcional, não aceita `null`. Campos achatados do evento original.

#### Valores possíveis

- `type`: `join`, `leave`, `live.update`, `message.meta`, `mute.change`

#### Observações

- Sem observações adicionais.

### `presence.updated`

Atualizacao de presenca de usuario ou presenca em chat.

**Flag:** `presenceUpdated`

**Eventos internos:** `*events.ChatPresence`, `*events.Presence`

**Persistência:** Nao persiste dados especificos antes da entrega do webhook.

**Tipo de `data`:** `object`

**DTO/normalizador:** `PresenceUpdatedWebhookData`

**Campos dinâmicos:** sim

**Implementado em:** `internal/whatsapp/service.go`, `internal/whatsapp/webhook_events.go`

#### Requisição

```http
POST /webhooks/codechat HTTP/1.1
Content-Type: application/json
x-webhook-event: presence.updated
```

#### Corpo

```json
{
  "data": {
    "chatJid": "5511999999999@s.whatsapp.net",
    "dateTime": "2026-07-04T18:00:00Z",
    "media": "text",
    "senderJid": "5511999999999@s.whatsapp.net",
    "state": "composing"
  },
  "event": "presence.updated",
  "instance": {
    "connectionStatus": "online",
    "externalAttributes": {},
    "id": 1,
    "name": "codechat",
    "ownerJid": "5511999999999@s.whatsapp.net"
  },
  "timestamp": "2026-07-04T18:00:00Z"
}
```

#### Campos de `data`

- `type`: `string`, opcional, não aceita `null`. Tipo fixo presence no payload vindo de *events.Presence. Valores possíveis: `presence`.
- `chatJid`: `string`, opcional, não aceita `null`. JID do chat no payload de ChatPresence.
- `senderJid`: `string`, opcional, não aceita `null`. JID do remetente no payload de ChatPresence.
- `state`: `string`, opcional, não aceita `null`. Estado de presenca no payload de ChatPresence.
- `media`: `string`, opcional, não aceita `null`. Tipo de midia quando presenca esta relacionada a midia.
- `jid`: `string`, opcional, não aceita `null`. JID no payload de Presence.
- `unavailable`: `boolean`, opcional, não aceita `null`. Indica indisponibilidade no payload de Presence.
- `lastSeen`: `string`, opcional, não aceita `null`. Ultimo visto quando informado.
- `dateTime`: `string`, obrigatório, não aceita `null`. Timestamp RFC3339 do processamento.

#### Valores possíveis

- `type`: `presence`

#### Observações

- O formato varia entre ChatPresence e Presence; use os campos presentes no payload recebido.

### `profile.picture.update`

Atualizacao de foto de perfil da propria instancia ou de outro JID.

**Flag:** `profilePictureUpdated`

**Eventos internos:** `*events.Picture`

**Persistência:** Quando o JID e a propria instancia, atualiza profilePicUrl da instancia antes da entrega. Para outros JIDs nao ha persistencia especifica.

**Tipo de `data`:** `object`

**DTO/normalizador:** `ProfilePictureUpdatedWebhookData`

**Campos dinâmicos:** não

**Implementado em:** `internal/whatsapp/service.go`, `internal/webhook/payload.go`

#### Requisição

```http
POST /webhooks/codechat HTTP/1.1
Content-Type: application/json
x-webhook-event: profile.picture.update
```

#### Corpo

```json
{
  "data": {
    "author": "5531999999999@s.whatsapp.net",
    "dateTime": "2026-07-04T18:00:00Z",
    "isGroup": true,
    "jid": "120363000000000000@g.us",
    "pictureId": "pic-123",
    "remove": false
  },
  "event": "profile.picture.update",
  "instance": {
    "connectionStatus": "online",
    "externalAttributes": {},
    "id": 1,
    "name": "codechat",
    "ownerJid": "5511999999999@s.whatsapp.net"
  },
  "timestamp": "2026-07-04T18:00:00Z"
}
```

#### Campos de `data`

- `jid`: `string`, obrigatório, não aceita `null`. JID que teve a foto alterada.
- `author`: `string`, opcional, não aceita `null`. JID do autor quando informado; omitido quando vazio.
- `dateTime`: `string`, obrigatório, não aceita `null`. Timestamp RFC3339 UTC do evento ou do processamento.
- `remove`: `boolean`, obrigatório, não aceita `null`. Indica remocao de foto.
- `pictureId`: `string`, opcional, não aceita `null`. ID da foto quando informado; omitido quando vazio.
- `isGroup`: `boolean`, obrigatório, não aceita `null`. Indica se o JID pertence a grupo.

#### Observações

- Sem observações adicionais.

### `qrcode.updated`

Novo QR Code disponivel para pareamento da instancia.

**Flag:** `qrcodeUpdated`

**Eventos internos:** `QR channel`

**Persistência:** Atualiza o status da instancia para qr_code antes da entrega.

**Tipo de `data`:** `object`

**DTO/normalizador:** `QRCodeUpdatedWebhookData`

**Campos dinâmicos:** não

**Implementado em:** `internal/whatsapp/service.go`, `internal/webhook/manager.go`

#### Requisição

```http
POST /webhooks/codechat HTTP/1.1
Content-Type: application/json
x-webhook-event: qrcode.updated
```

#### Corpo

```json
{
  "data": {
    "base64": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAA",
    "code": "2@abc",
    "count": 1,
    "expiresAt": "2026-07-04T18:01:00Z",
    "expiresInSeconds": 60
  },
  "event": "qrcode.updated",
  "instance": {
    "connectionStatus": "qr_code",
    "externalAttributes": {},
    "id": 1,
    "name": "codechat",
    "ownerJid": null
  },
  "timestamp": "2026-07-04T18:00:00Z"
}
```

#### Campos de `data`

- `count`: `number`, obrigatório, não aceita `null`. Quantidade de QRs emitidos nesta tentativa.
- `code`: `string`, obrigatório, não aceita `null`. Codigo bruto do QR Code.
- `base64`: `string`, obrigatório, não aceita `null`. Imagem do QR Code em data URL base64.
- `expiresInSeconds`: `number`, obrigatório, não aceita `null`. Tempo restante informado pelo canal de QR, em segundos.
- `expiresAt`: `string`, obrigatório, não aceita `null`. Timestamp RFC3339 UTC calculado para expiracao do QR Code.

#### Observações

- Evento emitido pelo fluxo de QR, nao diretamente por um struct de evento do whatsmeow.

### `send.message`

Mensagem enviada pela API apos envio e persistencia bem-sucedidos.

**Flag:** `sendMessage`

**Eventos internos:** `message service send result`

**Persistência:** Persistida antes da entrega pelo fluxo de envio de mensagens da API.

**Tipo de `data`:** `object`

**DTO/normalizador:** `MessageWebhookData`

**Campos dinâmicos:** não

**Implementado em:** `internal/message/service.go`, `internal/message/audio.go`, `internal/webhook/payload.go`

#### Requisição

```http
POST /webhooks/codechat HTTP/1.1
Content-Type: application/json
x-webhook-event: send.message
```

#### Corpo

```json
{
  "data": {
    "content": {
      "text": "Mensagem enviada"
    },
    "device": "web",
    "id": 2048,
    "isGroup": false,
    "keyFromMe": true,
    "keyLid": null,
    "keyParticipant": null,
    "keyParticipantLid": null,
    "keyRemoteJid": "5511999999999@s.whatsapp.net",
    "messageTimestamp": 1783188000,
    "messageType": "conversation",
    "metadata": null,
    "pushName": null
  },
  "event": "send.message",
  "instance": {
    "connectionStatus": "online",
    "externalAttributes": {},
    "id": 1,
    "name": "codechat",
    "ownerJid": "5511999999999@s.whatsapp.net"
  },
  "timestamp": "2026-07-04T18:00:00Z"
}
```

#### Campos de `data`

- `id`: `number`, obrigatório, não aceita `null`. ID interno da mensagem.
- `keyRemoteJid`: `string | null`, obrigatório, aceita `null`. JID remoto da mensagem.
- `keyLid`: `string | null`, obrigatório, aceita `null`. LID remoto da mensagem.
- `keyFromMe`: `boolean`, obrigatório, não aceita `null`. Indica se a mensagem foi enviada pela propria instancia.
- `keyParticipant`: `string | null`, obrigatório, aceita `null`. Participante em mensagens de grupo.
- `keyParticipantLid`: `string | null`, obrigatório, aceita `null`. LID do participante em mensagens de grupo.
- `pushName`: `string | null`, obrigatório, aceita `null`. Nome exibido do remetente quando conhecido.
- `messageType`: `string`, obrigatório, não aceita `null`. Tipo normalizado da mensagem.
- `content`: `object`, obrigatório, não aceita `null`. Conteudo normalizado da mensagem.
- `messageTimestamp`: `number`, obrigatório, não aceita `null`. Timestamp Unix em segundos.
- `device`: `string | null`, obrigatório, aceita `null`. Dispositivo/origem inferida da mensagem.
- `isGroup`: `boolean`, obrigatório, não aceita `null`. Indica se a mensagem pertence a grupo.
- `metadata`: `object | null`, obrigatório, aceita `null`. Metadados adicionais normalizados.

#### Observações

- Usa o mesmo DTO de messages.upsert, mas a origem e o envio pela propria API.
- Quando uma mensagem e aceita com options.mentionAll=true, o mesmo evento send.message tambem entrega o resultado definitivo do processamento assincrono. Nesse caso, data contem processId, status, mentionAll, externalAttributes e, em caso de sucesso, data.messageId, data.remoteJid, data.participantCount e data.timestamp. Em caso de falha, contem error.code e error.message.

### `settings.update`

Atualizacao de configuracoes do usuario/instancia.

**Flag:** `settingsUpdated`

**Eventos internos:** `*events.PushNameSetting`, `*events.UserStatusMute`

**Persistência:** Nao persiste dados especificos antes da entrega do webhook.

**Tipo de `data`:** `object`

**DTO/normalizador:** `SettingsUpdatedWebhookData`

**Campos dinâmicos:** não

**Implementado em:** `internal/whatsapp/service.go`, `internal/whatsapp/webhook_events.go`, `internal/webhook/payload.go`

#### Requisição

```http
POST /webhooks/codechat HTTP/1.1
Content-Type: application/json
x-webhook-event: settings.update
```

#### Corpo

```json
{
  "data": {
    "dateTime": "2026-07-04T18:00:00Z",
    "fromFullSync": false,
    "name": "Minha instancia",
    "type": "push.name"
  },
  "event": "settings.update",
  "instance": {
    "connectionStatus": "online",
    "externalAttributes": {},
    "id": 1,
    "name": "codechat",
    "ownerJid": "5511999999999@s.whatsapp.net"
  },
  "timestamp": "2026-07-04T18:00:00Z"
}
```

#### Campos de `data`

- `type`: `string`, obrigatório, não aceita `null`. Subtipo de configuracao. Valores possíveis: `push.name`, `user.status.mute`.
- `jid`: `string`, opcional, não aceita `null`. JID afetado quando o subtipo informar.
- `name`: `string`, opcional, não aceita `null`. Nome configurado no subtipo push.name.
- `muted`: `boolean`, opcional, não aceita `null`. Estado de mute no subtipo user.status.mute.
- `fromFullSync`: `boolean`, obrigatório, não aceita `null`. Indica se veio de sincronizacao completa.
- `dateTime`: `string`, obrigatório, não aceita `null`. Timestamp RFC3339 UTC do evento ou do processamento.

#### Valores possíveis

- `type`: `push.name`, `user.status.mute`

#### Observações

- Sem observações adicionais.

### `status.instance`

Eventos de estado operacional ou avisos da instancia.

**Flag:** `statusInstance`

**Eventos internos:** `*events.ClientOutdated`, `*events.TemporaryBan`, `*events.OfflineSyncPreview`, `*events.OfflineSyncCompleted`, `*events.PrivacySettings`, `*events.AppState`, `*events.AppStateSyncComplete`, `*events.AppStateSyncError`, `*events.AccountReachoutTimelock`

**Persistência:** Nao persiste dados especificos antes da entrega do webhook.

**Tipo de `data`:** `object`

**DTO/normalizador:** `InstanceStatusWebhookData`

**Campos dinâmicos:** sim

**Implementado em:** `internal/whatsapp/service.go`, `internal/webhook/payload.go`

#### Requisição

```http
POST /webhooks/codechat HTTP/1.1
Content-Type: application/json
x-webhook-event: status.instance
```

#### Corpo

```json
{
  "data": {
    "data": {
      "count": 185
    },
    "status": "completed",
    "type": "offline.sync.completed"
  },
  "event": "status.instance",
  "instance": {
    "connectionStatus": "online",
    "externalAttributes": {},
    "id": 1,
    "name": "codechat",
    "ownerJid": "5511999999999@s.whatsapp.net"
  },
  "timestamp": "2026-07-04T18:00:00Z"
}
```

#### Campos de `data`

- `type`: `string`, obrigatório, não aceita `null`. Subtipo do status da instancia.
- `status`: `string`, opcional, não aceita `null`. Status textual do subtipo; omitido quando vazio.
- `message`: `string`, opcional, não aceita `null`. Mensagem tecnica ou humana; omitida quando vazia.
- `data`: `object`, opcional, não aceita `null`. Dados adicionais do subtipo; omitido quando ausente.

#### Valores possíveis

- `type`: `client.outdated`, `temporary.ban`, `offline.sync.preview`, `offline.sync.completed`, `privacy.settings`, `app.state`, `app.state.sync.completed`, `app.state.sync.error`, `account.reachout.timelock`

#### Observações

- Sem observações adicionais.

### `user.about.update`

Atualizacao do recado/about de um usuario.

**Flag:** `userAboutUpdated`

**Eventos internos:** `*events.UserAbout`

**Persistência:** Nao persiste dados especificos antes da entrega do webhook.

**Tipo de `data`:** `object`

**DTO/normalizador:** `UserAboutUpdatedWebhookData`

**Campos dinâmicos:** não

**Implementado em:** `internal/whatsapp/service.go`, `internal/webhook/payload.go`

#### Requisição

```http
POST /webhooks/codechat HTTP/1.1
Content-Type: application/json
x-webhook-event: user.about.update
```

#### Corpo

```json
{
  "data": {
    "dateTime": "2026-07-04T18:00:00Z",
    "jid": "5511999999999@s.whatsapp.net",
    "status": "Disponivel"
  },
  "event": "user.about.update",
  "instance": {
    "connectionStatus": "online",
    "externalAttributes": {},
    "id": 1,
    "name": "codechat",
    "ownerJid": "5511999999999@s.whatsapp.net"
  },
  "timestamp": "2026-07-04T18:00:00Z"
}
```

#### Campos de `data`

- `jid`: `string`, obrigatório, não aceita `null`. JID do usuario.
- `status`: `string`, opcional, não aceita `null`. Texto do about quando informado.
- `dateTime`: `string`, obrigatório, não aceita `null`. Timestamp RFC3339 do processamento.

#### Observações

- Sem observações adicionais.

## Eventos não suportados ou ignorados

| Evento interno | Status | Motivo |
| --- | --- | --- |
| `PairPasskeyConfirmation` | `intentionally_ignored` | Evento interativo de pareamento com codigo; nao e contrato de webhook. |
| `PairPasskeyError` | `handled_without_webhook` | Tratado como estado/log de pareamento; nao ha payload publico estavel. |
| `PairPasskeyRequest` | `intentionally_ignored` | Contem desafio/chave publica de pareamento e nao e serializado para webhook. |
| `QRScannedWithoutMultidevice` | `handled_without_webhook` | O canal de QR converte o caso em falha de pareamento; emissoes diretas futuras caem em log fallback. |
| `MediaRetryError` | `internal_only` | Struct auxiliar de erro usada dentro do payload de media.retry. |
| `MexNotificationData` | `internal_only` | Struct auxiliar para notificacoes MEX sem evento publico dedicado. |
| `NewsletterMessageMeta` | `internal_only` | Struct auxiliar usada dentro de news.letter. |
