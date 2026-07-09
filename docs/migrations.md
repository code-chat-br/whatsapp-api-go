# Migrations

As migrations da aplicação e as migrations internas do Whatsmeow podem ser executadas sem iniciar o listener HTTP com:

```bash
go run ./cmd/migrate
```

No PowerShell:

```powershell
$env:SERVER_PORT = "8084"
$env:LOG_LEVEL = "trace"
go run ./cmd/migrate
```

O comando carrega a configuração pelo mesmo fluxo da API. Fora do Docker, `DOCKER_ENV=false` faz o processo carregar `.env`; dentro do Docker, defina `DOCKER_ENV=true` e forneça as variáveis diretamente.

O runner aplica somente arquivos `*.up.sql` em `internal/database/migrations`, em ordem lexicográfica, e registra os nomes aplicados na tabela `schema_migrations`. Depois disso, inicializa o `sqlstore` configurado para o Whatsmeow; `sqlstore.New` executa as migrations internas no backend selecionado.

O comando não inicia o Fiber, não registra rotas e não tenta restaurar conexões WhatsApp.
