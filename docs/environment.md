# Configuração de ambiente

| Variável | Obrigatória | Exemplo | Descrição |
| --- | ---: | --- | --- |
| `SERVER_PORT` | Não | `8084` | Porta do listener HTTP. A aplicação escuta em `:SERVER_PORT`. O padrão é `8084`. |
| `LOG_LEVEL` | Não | `trace` | Nível global do Zerolog. Valores aceitos: `trace`, `debug`, `info`, `warn`, `error`, `fatal`, `panic`, `disabled`. O padrão é `info`. |
| `DATABASE_URL` | Sim | `postgres://...` | String de conexão com o PostgreSQL. |
| `DATABASE_SAVE_DATA_NEW_MESSAGE` | Não | `true` | Habilita a persistência de linhas WhatsApp `Message` a partir de eventos de mensagens recebidas. O padrão é `true`. |
| `DATABASE_SAVE_MESSAGE_UPDATE` | Não | `false` | Habilita a persistência de eventos de recibo do WhatsApp em `MessageUpdate`. O padrão é `false`. |
| `DATABASE_SAVE_DATA_CONTACTS` | Não | `false` | Habilita a persistência de sincronização e eventos de contatos do WhatsApp em `Contact`. O padrão é `false`. |
| `WHATSAPP_SESSION_STORE` | Não | `postgres` | Banco usado pelo whatsmeow para sessões e dispositivos. Valores aceitos: `sqlite`, `postgres`. O padrão é `postgres` quando não for definido. |
| `WHATSAPP_SESSION_SQLITE_DSN` | Não | `file:./data/whatsmeow.db?_foreign_keys=on` | DSN do SQLite usado somente quando `WHATSAPP_SESSION_STORE=sqlite`. As chaves estrangeiras precisam continuar habilitadas. |
| `WHATSAPP_SESSION_POSTGRES_URL` | Não | `postgres://...` | String de conexão opcional e dedicada do PostgreSQL para as sessões do whatsmeow. Quando vazia, `DATABASE_URL` é usada pelo SQL store do whatsmeow. |
| `WEBHOOK_GLOBAL_URL` | Não | `https://example.com/webhook` | URL do webhook global. Precisa ser `http` ou `https` absoluta; obrigatória somente quando `WEBHOOK_GLOBAL_ENABLED=true`. |
| `WEBHOOK_GLOBAL_ENABLED` | Não | `false` | Habilita o envio de todos os eventos reconhecidos de todas as instâncias para `WEBHOOK_GLOBAL_URL`. O padrão é `false`. |
| `AUTHENTICATION_JWT_EXPIRES_IN` | Sim | `3600` | Expiração do JWT em segundos. O valor `0` remove a claim `exp`. |
| `AUTHENTICATION_JWT_SECRET` | Sim | `strong-secret` | Chave secreta usada para assinar JWTs com HS256. |
| `AUTHENTICATION_GLOBAL_AUTH_TOKEN` | Sim | `admin-token` | Token usado somente para criar e listar instâncias. |
| `QRCODE_LIMIT` | Sim | `5` | Número máximo de QR codes servidos durante uma tentativa de pareamento. |
| `QRCODE_EXPIRATION_TIME` | Sim | `30` | Tempo máximo de vida do QR code em segundos. |
| `QRCODE_LIGHT_COLOR` | Sim | `#ffffff` | Cor clara usada na geração do QR PNG. |
| `QRCODE_DARK_COLOR` | Sim | `#198754` | Cor escura usada na geração do QR PNG. |
| `CONFIG_SESSION_PHONE_CLIENT` | Não | `DESKTOP` | Tipo de plataforma exibido nos dispositivos conectados do WhatsApp. O padrão é `DESKTOP`. Valores aceitos: `ALOHA`, `ANDROID_AMBIGUOUS`, `ANDROID_PHONE`, `ANDROID_TABLET`, `AR_DEVICE`, `AR_WRIST`, `CATALINA`, `CHROME`, `CLOUD_API`, `DESKTOP`, `EDGE`, `FIREFOX`, `IE`, `IOS_CATALYST`, `IOS_PHONE`, `IPAD`, `OHANA`, `OPERA`, `SAFARI`, `SMARTGLASSES`, `TCL_TV`, `UWP`, `VR`, `WEAR_OS`. |
| `CONFIG_SESSION_PHONE_NAME` | Não | `CodeChat` | Nome do sistema ou cliente exibido nos dispositivos conectados do WhatsApp. O padrão é `CodeChat`. |
| `WHATSAPP_PAIRING_TIMEOUT` | Não | `3m` | Timeout total do contexto de pareamento por QR. O padrão é `3m` quando não for definido. |
| `WHATSAPP_AUTO_RECONNECT` | Sim | `true` | Habilita a restauração, na inicialização, das sessões que deveriam estar online. |
| `WHATSAPP_STARTUP_RECONNECT_CONCURRENCY` | Sim | `5` | Número máximo de sessões WhatsApp restauradas em paralelo. |
| `WHATSAPP_CONNECT_TIMEOUT` | Sim | `30` | Timeout inicial de espera da conexão, em segundos. |
| `WHATSAPP_RECONNECT_INITIAL_DELAY` | Sim | `2` | Backoff inicial de reconexão, em segundos. |
| `WHATSAPP_RECONNECT_MAX_DELAY` | Sim | `60` | Backoff máximo de reconexão, em segundos. |
| `WHATSAPP_PROFILE_PICTURE_TIMEOUT` | Sim | `15` | Timeout para buscar foto de perfil, em segundos. |
| `WHATSAPP_ADDRESS_CACHE_TTL` | Não | `168h` | TTL dos mapeamentos em cache entre endereço WhatsApp e JID canônico. O padrão é `168h`. |
| `MESSAGE_PROCESSING_WORKERS` | Não | `4` | Número máximo de jobs assíncronos de mensagem processados em paralelo. O padrão é `4`. |
| `MESSAGE_PROCESSING_QUEUE_SIZE` | Não | `100` | Número máximo de jobs assíncronos de mensagem aguardando em memória. O padrão é `100`; os valores precisam ser maiores que zero. |
| `MESSAGE_PROCESSING_TIMEOUT` | Não | `60s` | Timeout total para um job assíncrono de mensagem. O padrão é `60s`. |
| `MESSAGE_GROUP_INFO_TIMEOUT` | Não | `30s` | Timeout para carregar informações e participantes de grupo do WhatsApp durante o processamento de `mentionAll`. O padrão é `30s`. |
| `MESSAGE_SEND_TIMEOUT` | Não | `30s` | Timeout para presença/delay e envio final pelo WhatsApp durante o processamento assíncrono de mensagem. O padrão é `30s`. |

## Execução local

```bash
cp .env.dev .env
go run ./cmd/...
```

Quando `DOCKER_ENV` está ausente ou definido como `false`, a aplicação carrega `.env` e depois lê os valores do ambiente do processo. Variáveis já definidas no processo têm prioridade sobre os valores em `.env`.

`.env.dev` é um arquivo de referência para desenvolvimento local e não é carregado automaticamente.

## Store de sessão do Whatsmeow

`DATABASE_URL` continua sendo o banco principal da API. `WHATSAPP_SESSION_POSTGRES_URL` é opcional e é usado somente pelo SQL store do whatsmeow quando estiver preenchido. Os repositórios da API e as migrations sempre usam `DATABASE_URL`.

O SQLite armazena sessões em um arquivo local e exige armazenamento persistente em containers. O DSN padrão mantém as chaves estrangeiras do SQLite habilitadas:

```env
WHATSAPP_SESSION_STORE="sqlite"
WHATSAPP_SESSION_SQLITE_DSN="file:./data/whatsmeow.db?_foreign_keys=on"
WHATSAPP_SESSION_POSTGRES_URL=""
```

Quando Postgres é selecionado e `WHATSAPP_SESSION_POSTGRES_URL` está vazio, o whatsmeow usa o mesmo servidor/banco PostgreSQL configurado em `DATABASE_URL`, mas ainda com sua própria conexão SQL e seu próprio ciclo de vida:

```env
DATABASE_URL="postgresql://api:password@postgres:5432/codechat"

WHATSAPP_SESSION_STORE="postgres"
WHATSAPP_SESSION_POSTGRES_URL=""
```

Quando `WHATSAPP_SESSION_POSTGRES_URL` está preenchido, as sessões do whatsmeow são inicializadas e migradas somente nesse banco dedicado. A aplicação não volta para `DATABASE_URL` se a URL dedicada estiver inválida ou indisponível:

```env
DATABASE_URL="postgresql://api:password@postgres:5432/codechat"

WHATSAPP_SESSION_STORE="postgres"
WHATSAPP_SESSION_POSTGRES_URL="postgresql://sessions:password@postgres:5432/codechat_sessions"
```

Alterar `WHATSAPP_SESSION_STORE` não migra sessões existentes automaticamente. Os dispositivos só ficam disponíveis no novo backend se os dados do whatsmeow tiverem sido migrados antes; caso contrário, pode ser necessário parear as instâncias novamente. O backend anterior não é apagado.

## Execução com Docker

As variáveis precisam ser fornecidas diretamente ao container:

```yaml
environment:
  SERVER_PORT: "${SERVER_PORT}"
  LOG_LEVEL: "${LOG_LEVEL}"
  DATABASE_URL: "${DATABASE_URL}"
  DATABASE_SAVE_DATA_NEW_MESSAGE: "${DATABASE_SAVE_DATA_NEW_MESSAGE:-true}"
  DATABASE_SAVE_MESSAGE_UPDATE: "${DATABASE_SAVE_MESSAGE_UPDATE:-false}"
  DATABASE_SAVE_DATA_CONTACTS: "${DATABASE_SAVE_DATA_CONTACTS:-false}"
  WHATSAPP_SESSION_STORE: "${WHATSAPP_SESSION_STORE:-postgres}"
  WHATSAPP_SESSION_SQLITE_DSN: "${WHATSAPP_SESSION_SQLITE_DSN:-file:./data/whatsmeow.db?_foreign_keys=on}"
  WHATSAPP_SESSION_POSTGRES_URL: "${WHATSAPP_SESSION_POSTGRES_URL:-}"
  WEBHOOK_GLOBAL_URL: "${WEBHOOK_GLOBAL_URL:-}"
  WEBHOOK_GLOBAL_ENABLED: "${WEBHOOK_GLOBAL_ENABLED:-false}"
  AUTHENTICATION_JWT_EXPIRES_IN: "${AUTHENTICATION_JWT_EXPIRES_IN}"
  AUTHENTICATION_JWT_SECRET: "${AUTHENTICATION_JWT_SECRET}"
  AUTHENTICATION_GLOBAL_AUTH_TOKEN: "${AUTHENTICATION_GLOBAL_AUTH_TOKEN}"
  QRCODE_LIMIT: "${QRCODE_LIMIT}"
  QRCODE_EXPIRATION_TIME: "${QRCODE_EXPIRATION_TIME}"
  QRCODE_LIGHT_COLOR: "${QRCODE_LIGHT_COLOR}"
  QRCODE_DARK_COLOR: "${QRCODE_DARK_COLOR}"
  CONFIG_SESSION_PHONE_CLIENT: "${CONFIG_SESSION_PHONE_CLIENT:-DESKTOP}"
  CONFIG_SESSION_PHONE_NAME: "${CONFIG_SESSION_PHONE_NAME:-CodeChat}"
  WHATSAPP_PAIRING_TIMEOUT: "${WHATSAPP_PAIRING_TIMEOUT}"
  WHATSAPP_AUTO_RECONNECT: "${WHATSAPP_AUTO_RECONNECT}"
  WHATSAPP_STARTUP_RECONNECT_CONCURRENCY: "${WHATSAPP_STARTUP_RECONNECT_CONCURRENCY}"
  WHATSAPP_CONNECT_TIMEOUT: "${WHATSAPP_CONNECT_TIMEOUT}"
  WHATSAPP_RECONNECT_INITIAL_DELAY: "${WHATSAPP_RECONNECT_INITIAL_DELAY}"
  WHATSAPP_RECONNECT_MAX_DELAY: "${WHATSAPP_RECONNECT_MAX_DELAY}"
  WHATSAPP_PROFILE_PICTURE_TIMEOUT: "${WHATSAPP_PROFILE_PICTURE_TIMEOUT}"
  WHATSAPP_ADDRESS_CACHE_TTL: "${WHATSAPP_ADDRESS_CACHE_TTL:-168h}"
  MESSAGE_PROCESSING_WORKERS: "${MESSAGE_PROCESSING_WORKERS:-4}"
  MESSAGE_PROCESSING_QUEUE_SIZE: "${MESSAGE_PROCESSING_QUEUE_SIZE:-100}"
  MESSAGE_PROCESSING_TIMEOUT: "${MESSAGE_PROCESSING_TIMEOUT:-60s}"
  MESSAGE_GROUP_INFO_TIMEOUT: "${MESSAGE_GROUP_INFO_TIMEOUT:-30s}"
  MESSAGE_SEND_TIMEOUT: "${MESSAGE_SEND_TIMEOUT:-30s}"
```

Quando `DOCKER_ENV=true`, `.env` e `.env.dev` não são carregados.

Se `WHATSAPP_SESSION_STORE=sqlite`, monte um volume persistente para o diretório do SQLite. Para o DSN padrão, persista `/app/data` ou o caminho equivalente `data` do diretório de trabalho usado pela imagem:

```yaml
volumes:
  - whatsmeow_sessions:/app/data
```

`AUTHENTICATION_GLOBAL_AUTH_TOKEN` autentica somente `POST /instance/create` e `GET /instance/create`. Ele não autentica `GET /instance/fetchInstance/:instanceName` e não é aceito por `PUT /instance/refreshToken/:instanceName`.

O endpoint de refresh exige `Authorization: Bearer <token>` e o mesmo JWT atual no campo `oldToken` do corpo da requisição. Ele rotaciona o `Auth.token` armazenado para aquela instância, invalida imediatamente o token antigo e não representa um segundo tipo de refresh-token.

`AUTHENTICATION_JWT_EXPIRES_IN=0` remove completamente a claim `exp`. Ele não gera `exp: 0`.

Não use segredos de desenvolvimento em produção. JWTs, chaves de API, tokens globais, URLs de banco e segredos não devem ser escritos em logs.

`GET /instance/connect/:instanceName` retorna o primeiro QR code do WhatsApp como código bruto mais um PNG `data:image/png;base64,...` gerado com as cores de QR configuradas. O processo de pareamento continua depois da resposta HTTP e usa `WHATSAPP_PAIRING_TIMEOUT` como deadline total do contexto.

`GET /instance/connect/:instanceName/code/:phoneNumber` retorna exatamente o código de pareamento do Whatsmeow. Números de telefone são normalizados para dígitos antes de chamar o Whatsmeow.

Os dois endpoints de conexão exigem o bearer token da instância armazenado em `Auth.token`; o token global de admin não é aceito.

`CONFIG_SESSION_PHONE_CLIENT` e `CONFIG_SESSION_PHONE_NAME` são aplicados uma vez durante a inicialização, antes da criação do SQL Store e dos clientes do Whatsmeow. Eles afetam somente novos vínculos. Dispositivos já vinculados não são apagados, deslogados nem reescritos automaticamente; para ver um novo rótulo, desconecte a instância pelo fluxo existente, remova o dispositivo vinculado no telefone, reinicie a aplicação, gere um novo QR code e vincule novamente.
