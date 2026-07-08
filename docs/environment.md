# Environment configuration

| Variable | Required | Example | Description |
| --- | ---: | --- | --- |
| `DOCKER_ENV` | No | `false` | Defines whether `.env` is loaded. The default is `false`. |
| `SERVER_PORT` | No | `8084` | HTTP listener port. The app listens on `:SERVER_PORT`. Default is `8084`. |
| `LOG_LEVEL` | No | `trace` | Global Zerolog level. Accepted values: `trace`, `debug`, `info`, `warn`, `error`, `fatal`, `panic`, `disabled`. Default is `info`. |
| `DATABASE_URL` | Yes | `postgres://...` | PostgreSQL connection string. |
| `DATABASE_SAVE_DATA_NEW_MESSAGE` | No | `true` | Enables persistence of WhatsApp `Message` rows from incoming message events. Defaults to `true`. |
| `DATABASE_SAVE_MESSAGE_UPDATE` | No | `false` | Enables persistence of WhatsApp receipt events in `MessageUpdate`. Defaults to `false`. |
| `DATABASE_SAVE_DATA_CONTACTS` | No | `false` | Enables persistence of WhatsApp contact sync and contact events in `Contact`. Defaults to `false`. |
| `WHATSAPP_SESSION_STORE` | No | `postgres` | Database engine used by whatsmeow for sessions and devices. Accepted values: `sqlite`, `postgres`. Defaults to `postgres` when unset. |
| `WHATSAPP_SESSION_SQLITE_DSN` | No | `file:./data/whatsmeow.db?_foreign_keys=on` | SQLite DSN used only when `WHATSAPP_SESSION_STORE=sqlite`. Foreign keys must remain enabled. |
| `WHATSAPP_SESSION_POSTGRES_URL` | No | `postgres://...` | Optional dedicated PostgreSQL connection string for whatsmeow sessions. When empty, `DATABASE_URL` is used for the whatsmeow SQL store. |
| `WEBHOOK_GLOBAL_URL` | No | `https://example.com/webhook` | Global webhook URL. Must be absolute `http` or `https`; required only when `WEBHOOK_GLOBAL_ENABLED=true`. |
| `WEBHOOK_GLOBAL_ENABLED` | No | `false` | Enables sending every recognized event from every instance to `WEBHOOK_GLOBAL_URL`. Defaults to `false`. |
| `AUTHENTICATION_JWT_EXPIRES_IN` | Yes | `3600` | JWT expiration in seconds. The value `0` removes the `exp` claim. |
| `AUTHENTICATION_JWT_SECRET` | Yes | `strong-secret` | Secret key used to sign JWTs with HS256. |
| `AUTHENTICATION_GLOBAL_AUTH_TOKEN` | Yes | `admin-token` | Token used only to create and list instances. |
| `QRCODE_LIMIT` | Yes | `5` | Maximum QR codes served during one pairing attempt. |
| `QRCODE_EXPIRATION_TIME` | Yes | `30` | Maximum QR code lifetime in seconds. |
| `QRCODE_LIGHT_COLOR` | Yes | `#ffffff` | Light color used in QR PNG generation. |
| `QRCODE_DARK_COLOR` | Yes | `#198754` | Dark color used in QR PNG generation. |
| `CONFIG_SESSION_PHONE_CLIENT` | No | `DESKTOP` | Platform type shown in WhatsApp linked devices. Defaults to `DESKTOP`. Supported values: `ALOHA`, `ANDROID_AMBIGUOUS`, `ANDROID_PHONE`, `ANDROID_TABLET`, `AR_DEVICE`, `AR_WRIST`, `CATALINA`, `CHROME`, `CLOUD_API`, `DESKTOP`, `EDGE`, `FIREFOX`, `IE`, `IOS_CATALYST`, `IOS_PHONE`, `IPAD`, `OHANA`, `OPERA`, `SAFARI`, `SMARTGLASSES`, `TCL_TV`, `UWP`, `VR`, `WEAR_OS`. |
| `CONFIG_SESSION_PHONE_NAME` | No | `CodeChat` | System or client name shown in WhatsApp linked devices. Defaults to `CodeChat`. |
| `WHATSAPP_PAIRING_TIMEOUT` | No | `3m` | Total QR pairing context timeout. Defaults to `3m` when unset. |
| `WHATSAPP_AUTO_RECONNECT` | Yes | `true` | Enables startup restoration for desired-online sessions. |
| `WHATSAPP_STARTUP_RECONNECT_CONCURRENCY` | Yes | `5` | Maximum concurrent restored WhatsApp sessions. |
| `WHATSAPP_CONNECT_TIMEOUT` | Yes | `30` | Initial connection wait timeout in seconds. |
| `WHATSAPP_RECONNECT_INITIAL_DELAY` | Yes | `2` | Initial reconnect backoff in seconds. |
| `WHATSAPP_RECONNECT_MAX_DELAY` | Yes | `60` | Maximum reconnect backoff in seconds. |
| `WHATSAPP_PROFILE_PICTURE_TIMEOUT` | Yes | `15` | Profile picture retrieval timeout in seconds. |
| `WHATSAPP_ADDRESS_CACHE_TTL` | No | `168h` | TTL for cached WhatsApp address-to-canonical-JID mappings. Defaults to `168h`. |
| `MESSAGE_PROCESSING_WORKERS` | No | `4` | Maximum number of asynchronous message jobs processed in parallel. Defaults to `4`. |
| `MESSAGE_PROCESSING_QUEUE_SIZE` | No | `100` | Maximum number of asynchronous message jobs waiting in memory. Defaults to `100`; values must be greater than zero. |
| `MESSAGE_PROCESSING_TIMEOUT` | No | `60s` | Total timeout for one asynchronous message job. Defaults to `60s`. |
| `MESSAGE_GROUP_INFO_TIMEOUT` | No | `30s` | Timeout for loading WhatsApp group information and participants during `mentionAll` processing. Defaults to `30s`. |
| `MESSAGE_SEND_TIMEOUT` | No | `30s` | Timeout for presence/delay and final WhatsApp send during asynchronous message processing. Defaults to `30s`. |

## Local execution

```bash
cp .env.dev .env
go run ./cmd/...
```

When `DOCKER_ENV` is absent or set to `false`, the application loads `.env` and then reads values from the process environment. Variables already defined in the process have priority over values in `.env`.

`.env.dev` is a reference file for local development and is not loaded automatically.

## Whatsmeow session store

`DATABASE_URL` remains the main API database. `WHATSAPP_SESSION_POSTGRES_URL` is optional and is used only by the whatsmeow SQL store when it is filled. The API repositories and migrations always use `DATABASE_URL`.

SQLite stores sessions in a local file and requires persistent storage in containers. The default DSN keeps SQLite foreign keys enabled:

```env
WHATSAPP_SESSION_STORE="sqlite"
WHATSAPP_SESSION_SQLITE_DSN="file:./data/whatsmeow.db?_foreign_keys=on"
WHATSAPP_SESSION_POSTGRES_URL=""
```

When Postgres is selected and `WHATSAPP_SESSION_POSTGRES_URL` is empty, whatsmeow uses the same PostgreSQL server/database configured in `DATABASE_URL`, while still using its own SQL connection and lifecycle:

```env
DATABASE_URL="postgresql://api:password@postgres:5432/codechat"

WHATSAPP_SESSION_STORE="postgres"
WHATSAPP_SESSION_POSTGRES_URL=""
```

When `WHATSAPP_SESSION_POSTGRES_URL` is filled, whatsmeow sessions are initialized and migrated only in that dedicated database. The app does not fall back to `DATABASE_URL` if the dedicated URL is invalid or unavailable:

```env
DATABASE_URL="postgresql://api:password@postgres:5432/codechat"

WHATSAPP_SESSION_STORE="postgres"
WHATSAPP_SESSION_POSTGRES_URL="postgresql://sessions:password@postgres:5432/codechat_sessions"
```

Changing `WHATSAPP_SESSION_STORE` does not migrate existing sessions automatically. Devices are available in the new backend only if the whatsmeow data was migrated beforehand; otherwise instances may need to be paired again. The previous backend is not deleted.

## Docker execution

Variables must be provided directly to the container:

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

When `DOCKER_ENV=true`, `.env` and `.env.dev` are not loaded.

If `WHATSAPP_SESSION_STORE=sqlite`, mount a persistent volume for the SQLite directory. For the default DSN, persist `/app/data` or the equivalent working-directory `data` path used by the image:

```yaml
volumes:
  - whatsmeow_sessions:/app/data
```

`AUTHENTICATION_GLOBAL_AUTH_TOKEN` authenticates only `POST /instance/create` and `GET /instance/create`. It does not authenticate `GET /instance/fetchInstance/:instanceName` and is not accepted by `PUT /instance/refreshToken/:instanceName`.

The refresh endpoint requires `Authorization: Bearer <token>` and the same current JWT in the `oldToken` body field. It rotates the stored `Auth.token` for that instance, immediately invalidates the old token, and does not represent a second refresh-token type.

`AUTHENTICATION_JWT_EXPIRES_IN=0` removes the `exp` claim completely. It does not generate `exp: 0`.

Do not use development secrets in production. JWTs, API keys, global tokens, database URLs, and secrets must not be written to logs.

`GET /instance/connect/:instanceName` returns the first WhatsApp QR code as raw code plus a `data:image/png;base64,...` PNG generated with the configured QR colors. The pairing process continues after the HTTP response and uses `WHATSAPP_PAIRING_TIMEOUT` as the total context deadline.

`GET /instance/connect/:instanceName/code/:phoneNumber` returns the exact pairing code from Whatsmeow. Phone numbers are normalized to digits before calling Whatsmeow.

Both connection endpoints require the instance bearer token stored in `Auth.token`; the global admin token is not accepted.

`CONFIG_SESSION_PHONE_CLIENT` and `CONFIG_SESSION_PHONE_NAME` are applied once during startup before the Whatsmeow SQL Store and clients are created. They affect new links only. Existing linked devices are not deleted, logged out, or rewritten automatically; to see a new label, disconnect the instance through the existing flow, remove the linked device on the phone, restart the app, generate a new QR code, and link again.
