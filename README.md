# whatsapp-go-api

API HTTP em Go para gerenciar instâncias do WhatsApp com Whatsmeow.

![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)
[![Telegram Group](https://img.shields.io/badge/Group-Telegram-%2333C1FF)](https://t.me/codechatBR)
[![Whatsapp Group](https://img.shields.io/badge/Group-WhatsApp-%2322BC18)](https://chat.whatsapp.com/HyO8X8K0bAo0bfaeW8bhY5)
[![License: AGPL v3](https://img.shields.io/badge/License-AGPL_v3-blue.svg)](./LICENSE)

## Visão geral

`whatsapp-go-api` expõe funcionalidades do WhatsApp por meio de uma API HTTP escrita em Go. O projeto usa [Whatsmeow](https://github.com/tulir/whatsmeow) para a integração com o WhatsApp e [Fiber v3](https://github.com/gofiber/fiber) para o servidor HTTP.

A aplicação permite criar e autenticar múltiplas instâncias, conectar uma sessão por QR Code ou código de pareamento, consultar estado de conexão, enviar mensagens, operar recursos de chat e grupo, persistir dados no PostgreSQL e encaminhar eventos por webhook.

Este projeto não usa a API oficial do WhatsApp.

## Principais recursos

- Gerenciamento de instâncias com token próprio por instância.
- Autenticação administrativa global por header de API key.
- Conexão por QR Code.
- Conexão por código de pareamento com telefone.
- Fluxo de pareamento por passkey.
- Estado de conexão, logout e remoção de instância.
- Envio de texto, link com preview, mídia por base64, mídia por upload, áudio no formato WhatsApp, contato, localização e reação.
- Opções de mensagem para presença, delay, citação e menções.
- Operações de chat para validação de números, leitura, arquivamento, exclusão, edição, foto de perfil, rejeição de chamada e download de mídia.
- Operações de grupo para criação, foto, convite, revogação de convite, participantes e saída.
- Webhooks por instância e webhook global opcional.
- Persistência opcional de mensagens, atualizações de mensagem e contatos.
- Persistência de sessões Whatsmeow em PostgreSQL ou SQLite.
- Reconexão automática de sessões persistidas no startup.
- Migrations SQL internas para o banco principal.
- Especificação OpenAPI estática para endpoints de mensagem.

## Tecnologias utilizadas

- Go `1.26`.
- Fiber `v3.4.0`.
- Whatsmeow `v0.0.0-20260630180629-b572e5bcb92b`.
- PostgreSQL via `pgx/v5`.
- SQLite via `go-sqlite3` para store de sessão Whatsmeow.
- `sqlc` para código de acesso a dados já gerado no repositório.
- `zerolog` para logs.
- `go-playground/validator` para validação.
- `golang-jwt/jwt/v5` para tokens por instância.
- `air` como ferramenta opcional de desenvolvimento, com `.air.toml` já presente.

## Estrutura do projeto

```text
.
|-- cmd/
|   |-- api/
|   |-- migrate/
|   `-- webhook-docs/
|-- docs/
|-- internal/
|   |-- app/
|   |-- authentication/
|   |-- chat/
|   |-- config/
|   |-- database/
|   |-- group/
|   |-- http/
|   |-- instance/
|   |-- message/
|   |-- webhook/
|   `-- whatsapp/
|-- tests/
|-- .air.toml
|-- .env.dev
|-- go.mod
|-- go.sum
`-- sqlc.yaml
```

Executáveis em `cmd/`:

- `cmd/api`: inicia a API HTTP.
- `cmd/migrate`: executa migrations do banco principal e inicializa as migrations da store Whatsmeow.
- `cmd/webhook-docs`: regenera `docs/webhooks.md` a partir do contrato interno de webhooks.

## Pré-requisitos

Para desenvolvimento local:

- Git, caso ainda precise clonar o projeto.
- Go compatível com `go 1.26`.
- PostgreSQL acessível pela variável `DATABASE_URL`.
- Opcionalmente Air para hot reload.
- Opcionalmente SQLite quando `WHATSAPP_SESSION_STORE=sqlite`.

Para produção:

- Go ou um binário compilado da API.
- PostgreSQL para o banco principal.
- Store de sessão Whatsmeow em PostgreSQL ou SQLite, conforme configuração.
- Variáveis de ambiente seguras para autenticação e conexão.

Para Docker:

- Docker Engine ou Docker Desktop com `docker compose`.
- BuildKit/Buildx para builds multiplataforma.
- PostgreSQL acessível pela rede usada pelo container.
- FFmpeg não precisa ser instalado no host quando a API roda pela imagem Docker; a imagem final já contém `ffmpeg` e `ffprobe`.

## Instalação do Go

Instale o Go pela página oficial: [go.dev/dl](https://go.dev/dl/).

- Windows: use o instalador `.msi` oficial e abra um novo terminal após a instalação.
- macOS: use o pacote oficial `.pkg` ou um gerenciador de pacotes de sua preferência.
- Linux: use o pacote oficial ou o repositório da sua distribuição.

Valide a instalação:

```bash
go version
```

O diretório de binários do Go precisa estar no `PATH` para comandos instalados com `go install`. Versões anteriores à diretiva `go 1.26` do `go.mod` não são suportadas por este projeto.

## Instalação do Air para desenvolvimento

O projeto possui `.air.toml`, então o Air pode ser usado como dependência opcional de desenvolvimento:

```bash
go install github.com/air-verse/air@latest
```

O binário normalmente é instalado em:

```bash
go env GOPATH
```

No Linux ou macOS, se `air` não estiver no `PATH`, adicione o diretório de binários do Go:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

Depois execute:

```bash
air
```

A configuração atual compila `./cmd/api/main.go` e gera o binário temporário em `tmp/main.exe`.

## Instalação do Docker

O repositório contém `Dockerfile`, `docker-compose.yml`, `.env.docker.example` e `scripts/build-and-push.sh` para executar e publicar a API em container.

Links oficiais:

- [Docker Engine](https://docs.docker.com/engine/install/)
- [Docker Desktop para Windows e macOS](https://docs.docker.com/desktop/)
- [Instalação em Linux](https://docs.docker.com/engine/install/#server)

Valide a instalação:

```bash
docker version
docker compose version
```

Use `docker compose`; não use o comando legado `docker-compose`.

## Instalação local da API

### Clonar o projeto

A URL remota do repositório não pôde ser identificada de forma confiável neste checkout local. Após clonar pela URL real do seu repositório ou fork, entre no diretório:

```bash
cd whatsapp-go-api
```

### Instalar dependências

```bash
go mod download
go mod verify
```

### Configurar variáveis de ambiente

Para execução local fora de Docker, a aplicação carrega `.env`. Use o arquivo de referência existente:

```bash
cp .env.dev .env
```

No Windows PowerShell:

```powershell
Copy-Item .env.dev .env
```

Variáveis mínimas para iniciar a API:

- `DOCKER_ENV=false`
- `SERVER_PORT`
- `DATABASE_URL`
- `AUTHENTICATION_JWT_EXPIRES_IN`
- `AUTHENTICATION_JWT_SECRET`
- `AUTHENTICATION_GLOBAL_AUTH_TOKEN`
- `QRCODE_LIMIT`
- `QRCODE_EXPIRATION_TIME`
- `QRCODE_LIGHT_COLOR`
- `QRCODE_DARK_COLOR`
- `WHATSAPP_AUTO_RECONNECT`
- `WHATSAPP_STARTUP_RECONNECT_CONCURRENCY`
- `WHATSAPP_CONNECT_TIMEOUT`
- `WHATSAPP_RECONNECT_INITIAL_DELAY`
- `WHATSAPP_RECONNECT_MAX_DELAY`
- `WHATSAPP_PROFILE_PICTURE_TIMEOUT`

Consulte a [documentação de variáveis de ambiente](./docs/environment.md) para a lista completa.

### Preparar o banco de dados

O banco principal da API é PostgreSQL, configurado por `DATABASE_URL`. Crie o banco antes de iniciar a aplicação ou executar migrations.

Exemplo com `psql`:

```bash
createdb whatsapp
```

Configure a URL no `.env`, por exemplo:

```dotenv
DATABASE_URL="postgres://postgres:postgres@localhost:5432/whatsapp?sslmode=disable"
```

A store de sessão Whatsmeow é escolhida por `WHATSAPP_SESSION_STORE`:

- `postgres`: usa `WHATSAPP_SESSION_POSTGRES_URL` quando preenchida, ou `DATABASE_URL` quando vazia.
- `sqlite`: usa `WHATSAPP_SESSION_SQLITE_DSN`, com valor padrão compatível com `file:./data/whatsmeow.db?_foreign_keys=on`.

### Executar migrations

A API executa as migrations no startup. Também existe um comando dedicado:

```bash
go run ./cmd/migrate
```

As migrations do banco principal ficam em `internal/database/migrations` e registram o estado em `schema_migrations`. Há arquivos `.down.sql`, mas o código atual não expõe comando de rollback nem comando de status.

Leia mais em [Migrations](./docs/migrations.md).

### Iniciar a aplicação

```bash
go run ./cmd/api
```

Com Air:

```bash
air
```

Por padrão, quando `SERVER_PORT` não é alterado, a API escuta em:

```text
http://localhost:8084
```

Rotas públicas de saúde:

```text
GET /health
GET /ready
```

### Compilar

Linux ou macOS:

```bash
go build -o ./api ./cmd/api
```

Windows PowerShell:

```powershell
go build -o .\api.exe .\cmd\api
```

## Docker

O container executa a API pelo entrypoint real `./cmd/api`, compilado como `/app/codechat-api`. A imagem final usa Alpine, instala `ffmpeg` e `ffprobe`, define `DOCKER_ENV=true`, roda como usuário não-root `app` e mantém as migrations em `/app/internal/database/migrations`, porque o runner atual lê `internal/database/migrations` do filesystem.

### Build local

Linux ou macOS:

```bash
docker build \
  --build-arg GO_VERSION="$(grep '^go ' go.mod | awk '{print $2}')" \
  --build-arg APP_NAME="whatsapp-go-api" \
  --build-arg APP_VERSION="dev" \
  --build-arg APP_DESCRIPTION="API HTTP em Go para gerenciar instancias do WhatsApp com Whatsmeow" \
  --build-arg APP_DEVELOPER="CodeChat" \
  --build-arg APP_REPOSITORY="$(git config --get remote.origin.url)" \
  --build-arg BUILD_DATE="$(date -u +'%Y-%m-%dT%H:%M:%SZ')" \
  --build-arg VCS_REF="$(git rev-parse HEAD)" \
  --build-arg VCS_URL="$(git config --get remote.origin.url)" \
  -t codechat-whatsapp-api:dev \
  .
```

PowerShell:

```powershell
$goVersion = (Select-String -Path go.mod -Pattern '^go ' | ForEach-Object { $_.Line.Split(' ')[1] })
$buildDate = (Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')
$vcsRef = git rev-parse HEAD
$vcsUrl = git config --get remote.origin.url

docker build `
  --build-arg GO_VERSION="$goVersion" `
  --build-arg APP_NAME="whatsapp-go-api" `
  --build-arg APP_VERSION="dev" `
  --build-arg APP_DESCRIPTION="API HTTP em Go para gerenciar instancias do WhatsApp com Whatsmeow" `
  --build-arg APP_DEVELOPER="CodeChat" `
  --build-arg APP_REPOSITORY="$vcsUrl" `
  --build-arg BUILD_DATE="$buildDate" `
  --build-arg VCS_REF="$vcsRef" `
  --build-arg VCS_URL="$vcsUrl" `
  -t codechat-whatsapp-api:dev `
  .
```

### Execução direta

```bash
docker run --rm \
  --env-file .env \
  -p 8084:8084 \
  codechat-whatsapp-api:dev
```

No Docker, mantenha `DOCKER_ENV=true`. A porta interna padrão é `8084`, lida de `SERVER_PORT`, e as rotas públicas de saúde continuam sendo:

```text
GET /health
GET /ready
```

### Docker Compose

Crie um arquivo `.env` a partir do exemplo e preencha as variáveis reais. Não versiona secrets.

```bash
cp .env.docker.example .env
docker compose up -d --build
```

Logs:

```bash
docker compose logs -f codechat-api
```

Encerrar:

```bash
docker compose down
```

O Compose não publica a porta no host por padrão; ele usa `expose` para o Traefik. Para teste local direto sem Traefik, use `docker run -p 8084:8084` ou um arquivo override próprio com `ports`.

### Traefik

Crie a rede externa somente se ela ainda não existir:

```bash
docker network create traefik_public
```

O serviço entra nas redes `codechat_network` e `traefik_network`. A rede externa é parametrizada por:

```env
TRAEFIK_NETWORK=traefik_public
```

Regra padrão:

```text
Host(`api.codechat.local`)
```

Variáveis principais:

```env
API_HOST=api.codechat.local
TRAEFIK_ENTRYPOINT=websecure
TRAEFIK_TLS=true
TRAEFIK_CERT_RESOLVER=letsencrypt
```

A porta configurada no load balancer Traefik é a mesma da aplicação:

```text
traefik.http.services.codechat-api.loadbalancer.server.port=8084
```

O arquivo base não adiciona labels vazias para middlewares ou `serversTransport`. Quando precisar referenciar middlewares externos, adicione-os em um override de produção, por exemplo:

```yaml
services:
  codechat-api:
    labels:
      - "traefik.http.routers.codechat-api.middlewares=compress@file,cors@file"
      - "traefik.http.services.codechat-api.loadbalancer.serverstransport=sse_transport@file"
```

Não configure timeouts no Traefik que encerrem conexões persistentes, porque a API usa conexões HTTP longas durante fluxos de WhatsApp e uploads.

### Dependência do FFmpeg

A imagem Docker contém:

```text
/usr/bin/ffmpeg
/usr/bin/ffprobe
```

Variáveis usadas dentro do container:

```env
FFMPEG_PATH=/usr/bin/ffmpeg
FFPROBE_PATH=/usr/bin/ffprobe
TMPDIR=/app/tmp
```

Fora do Docker, se `FFMPEG_PATH` e `FFPROBE_PATH` não forem definidas, a aplicação continua procurando `ffmpeg` e `ffprobe` no `PATH`.

Verificar no container:

```bash
docker compose exec codechat-api ffmpeg -version
docker compose exec codechat-api ffprobe -version
```

Instalação fora do Docker:

Ubuntu e Debian:

```bash
sudo apt-get update
sudo apt-get install -y ffmpeg
```

Alpine:

```bash
apk add --no-cache ffmpeg
```

macOS:

```bash
brew install ffmpeg
```

Windows:

```powershell
winget install --id Gyan.FFmpeg
```

Valide:

```bash
ffmpeg -version
ffprobe -version
```

### Build e push multiplataforma

O script publica imagens para `linux/amd64` e `linux/arm64` por padrão e exige uma tag de versão explícita.

```bash
chmod +x scripts/build-and-push.sh

DOCKERHUB_USERNAME=codechatbr \
DOCKER_IMAGE_NAME=whatsapp-go-api \
IMAGE_VERSION=v1.0.0 \
APP_NAME="whatsapp-go-api" \
APP_DEVELOPER="CodeChat" \
PUSH_LATEST=true \
PUSH_COMMIT_TAG=true \
./scripts/build-and-push.sh
```

Variáveis aceitas:

```text
DOCKERHUB_USERNAME
DOCKER_IMAGE_NAME
IMAGE_VERSION
GO_VERSION
APP_NAME
APP_DESCRIPTION
APP_DEVELOPER
APP_REPOSITORY
VCS_URL
PLATFORMS
BUILDER_NAME
PUSH_LATEST
PUSH_COMMIT_TAG
```

Verificar labels:

```bash
docker image inspect codechatbr/whatsapp-go-api:v1.0.0
```

Verificar manifesto:

```bash
docker buildx imagetools inspect \
  codechatbr/whatsapp-go-api:v1.0.0
```

## Configuração

As variáveis estão agrupadas nas seguintes áreas:

- Servidor HTTP: porta e modo de execução.
- Logs: nível de log.
- Banco principal: URL e persistência opcional de mensagens, atualizações e contatos.
- Autenticação: segredo JWT, expiração e token global.
- WhatsApp: QR Code, dispositivo vinculado, timeouts e reconexão.
- Sessões Whatsmeow: backend `postgres` ou `sqlite`.
- Webhooks: URL global e ativação global.
- Processamento de mensagens: workers, fila e timeouts.

Consulte a [documentação de variáveis de ambiente](./docs/environment.md).

## Documentação

A documentação técnica completa está disponível em [`/docs`](./docs/).

- [Variáveis de ambiente](./docs/environment.md): descreve execução local, execução em Docker e configuração da store de sessões Whatsmeow.
- [Migrations](./docs/migrations.md): resume o comando dedicado de migrations e o comportamento no startup.
- [Pareamento por Passkey](./docs/passkey-pairing.md): detalha endpoints, headers, estados e limitações do fluxo de passkey.
- [Envio de mensagens](./docs/send-messages.md): documenta corpos, respostas, opções, fila, erros e limitações dos endpoints de mensagem.
- [Webhooks](./docs/webhooks.md): descreve configuração, envelope, headers, entrega e payloads dos eventos suportados.
- [OpenAPI](./docs/openapi.yaml): especificação OpenAPI estática dos endpoints de envio de mensagem.

## Autenticação

A API usa dois escopos de autenticação.

Autenticação administrativa global:

- Protege criação e listagem de instâncias.
- Usa o valor de `AUTHENTICATION_GLOBAL_AUTH_TOKEN`.
- Headers aceitos: `apikey`, `x-api-key` e `apiKey`.
- Se mais de um desses headers for enviado, os valores precisam ser iguais.

Exemplo:

```http
apikey: <GLOBAL_API_KEY>
```

Autenticação por instância:

- Protege conexão, estado, logout, remoção, token refresh, webhooks, mensagens, chats e grupos.
- Usa JWT gerado na criação da instância.
- O token precisa pertencer ao `:instanceName` da rota.

Exemplo:

```http
Authorization: Bearer <INSTANCE_TOKEN>
```

## Uso básico da API

Os exemplos abaixo usam dados fictícios e assumem a API em `http://localhost:8084`.

### 1. Criar uma instância

```bash
curl -X POST "http://localhost:8084/instance/create" \
  -H "Content-Type: application/json" \
  -H "apikey: <GLOBAL_API_KEY>" \
  -d '{
    "instanceName": "minha-instancia",
    "description": "Instância de desenvolvimento"
  }'
```

A resposta inclui `auth.token`. Use esse valor como `<INSTANCE_TOKEN>`.

### 2. Conectar por QR Code

```bash
curl -X GET "http://localhost:8084/instance/connect/minha-instancia" \
  -H "Authorization: Bearer <INSTANCE_TOKEN>"
```

A resposta contém o código e o QR Code em base64 quando a instância ainda não está conectada.

### 3. Conectar por código de pareamento

```bash
curl -X GET "http://localhost:8084/instance/connect/minha-instancia/code/5511999999999" \
  -H "Authorization: Bearer <INSTANCE_TOKEN>"
```

### 4. Consultar o status da conexão

```bash
curl -X GET "http://localhost:8084/instance/connectionState/minha-instancia" \
  -H "Authorization: Bearer <INSTANCE_TOKEN>"
```

### 5. Enviar uma mensagem de texto

```bash
curl -X POST "http://localhost:8084/message/sendText/minha-instancia" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <INSTANCE_TOKEN>" \
  -d '{
    "number": "5511999999999",
    "textMessage": {
      "text": "Olá!"
    }
  }'
```

### 6. Configurar um webhook da instância

```bash
curl -X PUT "http://localhost:8084/webhook/set/minha-instancia" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <INSTANCE_TOKEN>" \
  -d '{
    "url": "https://example.com/webhooks/whatsapp",
    "enabled": true,
    "events": {
      "qrcodeUpdated": true,
      "connectionUpdated": true,
      "messagesUpsert": true,
      "sendMessage": true
    }
  }'
```

## OpenAPI

O projeto possui especificação OpenAPI estática em [docs/openapi.yaml](./docs/openapi.yaml). O servidor HTTP atual não registra rota de Swagger, Scalar ou Redoc.

## Envio de mensagens

Endpoints de mensagem implementados:

- `POST /message/sendText/:instanceName`
- `POST /message/sendLink/:instanceName`
- `POST /message/sendMedia/:instanceName`
- `POST /message/sendMediaFile/:instanceName`
- `POST /message/sendWhatsAppAudio/:instanceName`
- `POST /message/sendWhatsAppAudioFile/:instanceName`
- `POST /message/sendContact/:instanceName`
- `POST /message/sendLocation/:instanceName`
- `POST /message/sendReaction/:instanceName`

As opções aceitas incluem `delay`, `presence`, `quotedMessageId`, `quotedMessage`, `externalAttributes` e `mentionAll`. Para detalhes de payload, respostas e limitações, consulte [Envio de mensagens](./docs/send-messages.md).

## Webhooks

O webhook por instância é configurado por:

```text
PUT /webhook/set/:instanceName
GET /webhook/find/:instanceName
```

O corpo de configuração aceita:

- `url`: URL HTTP ou HTTPS de destino.
- `enabled`: habilita ou desabilita a entrega da instância.
- `events`: objeto com flags por evento, usando campos como `qrcodeUpdated`, `connectionUpdated`, `messagesUpsert` e `sendMessage`.

Também existe webhook global opcional por `WEBHOOK_GLOBAL_URL` e `WEBHOOK_GLOBAL_ENABLED`.

O envelope geral entregue é:

```json
{
  "event": "messages.upsert",
  "instance": {
    "id": 1,
    "name": "minha-instancia",
    "connectionStatus": "ONLINE",
    "ownerJid": "5511999999999@s.whatsapp.net",
    "externalAttributes": {}
  },
  "data": {},
  "timestamp": "2026-07-07T12:00:00Z"
}
```

Headers enviados:

- `Content-Type`
- `User-Agent`
- `x-request-id`
- `x-owner-jid`
- `x-instance-name`
- `x-instance-id`
- `x-webhook-event`

A entrega é assíncrona por fila em memória. Falhas são registradas em log; o código atual não implementa retry persistente.

Consulte [Webhooks](./docs/webhooks.md) para o mapa completo de eventos e payloads.

## Persistência de sessões

As sessões e dispositivos do WhatsApp são persistidos pela `sqlstore` do Whatsmeow.

Backend configurável:

```dotenv
WHATSAPP_SESSION_STORE="postgres"
```

Valores aceitos:

- `postgres`: usa `WHATSAPP_SESSION_POSTGRES_URL` ou, quando vazia, `DATABASE_URL`.
- `sqlite`: usa `WHATSAPP_SESSION_SQLITE_DSN`.

Quando `WHATSAPP_AUTO_RECONNECT=true`, o startup tenta restaurar as sessões persistidas. O número de restaurações concorrentes é controlado por `WHATSAPP_STARTUP_RECONNECT_CONCURRENCY`.

Consulte [Variáveis de ambiente](./docs/environment.md).

## Passkey

O projeto implementa endpoints de pareamento por passkey:

```text
POST /instance/connect/:instanceName/passkey/challenge
POST /instance/connect/:instanceName/passkey/assertion
```

O fluxo cria um challenge temporário para a instância e espera a assertion do cliente. Ele depende da mesma autenticação por instância via `Authorization: Bearer <INSTANCE_TOKEN>`.

Consulte [Pareamento por Passkey](./docs/passkey-pairing.md) para requisitos, estados, erros e limitações.

## Comandos de desenvolvimento

```bash
go mod download
go mod verify
go run ./cmd/migrate
go run ./cmd/api
go run ./cmd/webhook-docs
go test ./...
go vet ./...
go build ./...
air
```

Não há `Makefile` ou `Taskfile` no repositório atual.

## Testes

Execute:

```bash
go test ./...
```

Alguns testes e caminhos de inicialização dependem de configuração válida de ambiente e PostgreSQL quando exercitam banco real. A suíte existente também possui testes unitários para configuração, HTTP, mensagens, webhooks e WhatsApp.

## Solução de problemas

- Go em versão incompatível: confirme `go version` e use uma versão compatível com `go 1.26`.
- `air: command not found`: instale com `go install github.com/air-verse/air@latest` e adicione `$(go env GOPATH)/bin` ao `PATH`.
- `.env` ausente em execução local: copie `.env.dev` para `.env`.
- `DATABASE_URL` ausente ou inválida: confira a URL do PostgreSQL e o parâmetro `sslmode` quando necessário.
- PostgreSQL indisponível: valide host, porta, credenciais e existência do banco.
- Migrations pendentes ou falhando: execute `go run ./cmd/migrate` e confira a tabela `schema_migrations`.
- Porta em uso: altere `SERVER_PORT` no `.env`.
- Sessão não reconectada: confirme `WHATSAPP_AUTO_RECONNECT=true`, store de sessão correta e registros de estado da instância.
- QR Code expirado: solicite novamente `GET /instance/connect/:instanceName`.
- Token recusado: confirme se o header é `Authorization: Bearer <INSTANCE_TOKEN>` e se o token pertence ao mesmo `instanceName`.
- Webhook sem entrega: confira `enabled`, `events`, URL HTTP/HTTPS e logs de falha da fila.

## Segurança e uso responsável

Este projeto não é afiliado, patrocinado ou endossado pelo WhatsApp ou pela Meta. Ele usa uma integração não oficial via Whatsmeow, e alterações no protocolo do WhatsApp podem causar interrupções.

O usuário é responsável por cumprir leis, termos de serviço e políticas aplicáveis. Não use a API para spam, abuso ou envio de mensagens sem consentimento.

Cuidados operacionais:

- Não envie tokens, chaves, `.env` ou bancos de sessão ao Git.
- Use valores fortes para `AUTHENTICATION_JWT_SECRET` e `AUTHENTICATION_GLOBAL_AUTH_TOKEN`.
- Evite registrar tokens, números completos, conteúdos de mensagem ou dados sensíveis em logs.
- Proteja o acesso HTTP com rede privada, proxy autenticado ou controles equivalentes em produção.

## Licença

Este projeto é disponibilizado sob a licença [GNU Affero General Public License v3.0](./LICENSE), utilizando o identificador SPDX `AGPL-3.0-only`.

O uso comercial é permitido sob os termos da AGPL. Organizações que necessitem manter modificações privadas, incorporar o projeto em software proprietário ou utilizar termos diferentes podem solicitar uma [licença comercial separada](./LICENSE-COMMERCIAL.md).

Componentes de terceiros permanecem sujeitos às suas respectivas licenças. Consulte também os arquivos [NOTICE](./NOTICE) e [THIRD_PARTY_NOTICES.md](./THIRD_PARTY_NOTICES.md).

Este projeto não é oficial, não é afiliado à Meta ou ao WhatsApp, e o usuário é responsável por cumprir leis, termos de serviço e políticas aplicáveis.

## Contribuição

Leia [CONTRIBUTING.md](./CONTRIBUTING.md) antes de enviar contribuições. Contribuições externas poderão exigir aceite do [CLA.md](./CLA.md).
