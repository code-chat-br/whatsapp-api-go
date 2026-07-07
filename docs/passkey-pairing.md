# Pareamento por Passkey no WhatsApp

Este documento descreve o suporte da API ao pareamento de contas WhatsApp que exigem Passkey durante o fluxo de conexao com `whatsmeow`.

O worker Go continua sendo o proprietario do cliente WhatsApp. A extensao do navegador sera configurada separadamente e nao faz parte deste fluxo de API.

## Objetivo

Algumas contas do WhatsApp exigem uma assertion WebAuthn antes de permitir a vinculacao de um novo dispositivo. Como o worker e headless, ele busca o challenge com o `whatsmeow`, entrega esse challenge ao painel, recebe a assertion produzida no navegador do dono da conta e envia a resposta de volta pelo mesmo `*whatsmeow.Client`.

O challenge e a assertion sao efemeros:

- nao sao persistidos no banco;
- nao sao enviados para webhooks externos;
- nao devem aparecer em logs;
- a assertion aceita para processamento consome o challenge.

## Pre-requisitos

- A instancia deve existir e estar ativa.
- A requisicao deve usar o token bearer da propria instancia.
- Deve haver uma sessao de pareamento QR ativa para a instancia.
- O cliente WhatsApp precisa estar conectado e ainda nao logado.
- O mesmo processo precisa receber o challenge e a assertion, porque o cache interno de Passkey fica dentro do objeto `*whatsmeow.Client`.

## Endpoints

| Metodo | Caminho | Descricao |
| --- | --- | --- |
| `POST` | `/instance/connect/:instanceName/passkey/challenge` | Retorna ou cria um challenge WebAuthn para a sessao de pareamento ativa. |
| `POST` | `/instance/connect/:instanceName/passkey/assertion` | Recebe a assertion WebAuthn e envia para o WhatsApp. |

## Headers

| Header | Obrigatorio | Valor |
| --- | --- | --- |
| `Authorization` | Sim | `Bearer <instance-token>` |
| `Content-Type` | Sim para assertion | `application/json` |

## Solicitar challenge

```http
POST /instance/connect/codechat/passkey/challenge
Authorization: Bearer <instance-token>
```

Resposta `200 OK`:

```json
{
  "requestId": "7bbaf109-e0cc-44de-a434-8d48dfd5cb7b",
  "state": "AWAITING_ASSERTION",
  "expiresAt": "2026-07-06T18:30:00Z",
  "publicKey": {
    "challenge": "base64url-unpadded",
    "timeout": 300000,
    "rpId": "whatsapp.com",
    "allowCredentials": [
      {
        "id": "base64url-unpadded",
        "type": "public-key",
        "transports": ["internal", "hybrid"]
      }
    ],
    "userVerification": "required",
    "extensions": {}
  }
}
```

Se ja existir um challenge valido e nao consumido, o endpoint retorna o mesmo `requestId` e o mesmo `publicKey`. Isso evita multiplos challenges por clique duplicado.

## Enviar assertion

```http
POST /instance/connect/codechat/passkey/assertion
Authorization: Bearer <instance-token>
Content-Type: application/json
```

Body:

```json
{
  "requestId": "7bbaf109-e0cc-44de-a434-8d48dfd5cb7b",
  "assertion": {
    "id": "credential-id",
    "rawId": "base64url-unpadded",
    "type": "public-key",
    "response": {
      "clientDataJSON": "base64url-unpadded",
      "authenticatorData": "base64url-unpadded",
      "signature": "base64url-unpadded",
      "userHandle": null
    }
  }
}
```

Resposta `202 Accepted`:

```json
{
  "state": "AWAITING_CONFIRMATION",
  "message": "A assertion foi enviada ao WhatsApp."
}
```

O resultado final continua chegando pelo QR channel do `whatsmeow`: confirmacao de Passkey, `PairSuccess` e conexao online.

## Estados

| Estado | Significado |
| --- | --- |
| `IDLE` | Nenhum fluxo de Passkey ativo. |
| `FETCHING_CHALLENGE` | Worker buscando challenge no WhatsApp. |
| `AWAITING_ASSERTION` | Challenge disponivel, aguardando assertion do navegador. |
| `SUBMITTING_ASSERTION` | Assertion validada localmente e sendo enviada ao WhatsApp. |
| `AWAITING_CONFIRMATION` | WhatsApp recebeu a assertion e pode exigir aprovacao no telefone. |
| `CONFIRMATION_SENT` | Worker enviou `SendPasskeyConfirmation`. |
| `COMPLETED` | Pareamento concluido. |
| `FAILED` | Pareamento por Passkey falhou. |
| `EXPIRED` | Challenge expirou antes do uso. |

## Erros

O envelope segue o padrao atual da API:

```json
{
  "statusCode": 409,
  "error": "conflict",
  "messages": ["INVALID_PAIRING_STATE"]
}
```

| HTTP | Codigo |
| --- | --- |
| `404` | `PAIRING_SESSION_NOT_FOUND` |
| `409` | `PAIRING_SESSION_NOT_ACTIVE` |
| `409` | `INVALID_PAIRING_STATE` |
| `409` | `PASSKEY_REQUEST_MISMATCH` |
| `409` | `PASSKEY_CHALLENGE_ALREADY_USED` |
| `409` | `INSTANCE_ALREADY_CONNECTED` |
| `410` | `PASSKEY_CHALLENGE_EXPIRED` |
| `422` | `INVALID_PASSKEY_ASSERTION` |
| `422` | `PASSKEY_NOT_AVAILABLE` |
| `503` | `WHATSAPP_CLIENT_NOT_CONNECTED` |
| `503` | `PASSKEY_SERVICE_UNAVAILABLE` |

## Sequencia do fluxo

1. O painel inicia o fluxo existente de QR Code.
2. O worker cria o `*whatsmeow.Client`, chama `GetQRChannel` e conecta.
3. Quando o WhatsApp exige Passkey, o QR channel pode emitir `passkey-request`; se isso nao acontecer, o painel chama o endpoint de challenge.
4. O endpoint de challenge usa o mesmo `ManagedWhatsAppClient` e chama `DangerousInternals().GetPasskeyRequestOptions`.
5. O painel entrega `publicKey` para a extensao do navegador.
6. A extensao roda WebAuthn em `web.whatsapp.com` e devolve a assertion ao painel.
7. O painel envia a assertion para `/passkey/assertion`.
8. O worker valida `requestId`, estado, expiracao e uso unico, marca o challenge como consumido e chama `SendPasskeyResponse`.
9. O QR channel recebe `passkey-confirmation`. Se `SkipHandoffUX` for `false`, o worker chama `SendPasskeyConfirmation`; se for `true`, o proprio QR channel do `whatsmeow` ja confirmou.
10. O WhatsApp emite sucesso e o fluxo existente publica a instancia online.

## Base64url

Os campos `challenge`, `allowCredentials[].id`, `rawId`, `clientDataJSON`, `authenticatorData`, `signature` e `userHandle` usam base64url sem padding.

Nao converter para base64 padrao, nao adicionar `=`, nao trocar `-` por `+`, nao trocar `_` por `/` e nao decodificar/recodificar no painel. A API desserializa a assertion diretamente para `go.mau.fi/whatsmeow/types.WebAuthnResponse`.

## Mesmo cliente

Challenge, assertion e confirmacao precisam usar o mesmo `*whatsmeow.Client` que iniciou o QR channel. O `whatsmeow` mantem cache efemero dentro desse objeto durante o pareamento.

Nao ha endpoint manual de confirmacao. A confirmacao pertence ao worker que possui o cliente.

## Multiplas replicas

Este fluxo esta correto para execucao single-node ou para ambientes onde a instancia e roteada sempre para o node proprietario do `ManagedWhatsAppClient`.

Se o challenge for criado no node A e a assertion for enviada ao node B, o pareamento falhara, porque o node B nao possui o cache interno do `*whatsmeow.Client`. Redis ou banco de dados nao resolvem isso por si so, e o cache nao deve ser serializado.

Em ambientes com multiplas replicas, use afinidade/ownership por instancia para garantir que os dois endpoints de Passkey cheguem ao mesmo processo.

## Extensao

A extensao do navegador nao e instalada nem configurada por esta API. Ela deve apenas receber o `publicKey`, executar `navigator.credentials.get` no contexto correto e devolver a assertion sem transformar os campos base64url.
