# Jungle — serviço financeiro em Go

API HTTP e consumidor SQS para carteiras e apostas, com PostgreSQL, Keycloak/OIDC,
Uber Fx, ledger imutável, inbox e transactional outbox.

O domínio Money e Wallet foi desenvolvido inicialmente como exercício de aprendizado.
A integração foi acrescentada depois. O projeto continua sendo uma entrega parcial:
consulte as limitações em [ARCHITECTURE.md](ARCHITECTURE.md).

## Pré-requisitos

- Go 1.25 ou posterior (Dockerfile e go.mod usam 1.25.0).
- Docker Engine/Desktop em funcionamento, com containers Linux e Compose v2.
- Para `go test -race` fora do container, compilador C configurado. No Windows, o
  detector pode falhar se os headers do compilador não estiverem instalados.
- Portas locais: 5432, 4566, 8080 e 8081; para três instâncias, também 8082 e 8083.

Se `docker info` retorna erro, resolva o Docker/virtualização antes da integração.
A suíte unitária não depende de containers. O GitHub Actions executa a integração
real em Linux, incluindo três processos independentes.

## Inicializar tudo

```powershell
Copy-Item .env.example .env
docker compose up --build -d --wait --wait-timeout 240
Invoke-RestMethod http://localhost:8080/health/ready
```

O Compose fornece as variáveis aos containers; não é necessário preencher `.env`
para esse modo. As credenciais versionadas são exclusivamente de desenvolvimento.
O programa Go não lê `.env` automaticamente: para `go run`, exporte as variáveis
da seção abaixo no ambiente do processo.

O startup aplica a migration versionada sob advisory lock, importa o realm `jungle`
no Keycloak e provisiona as filas FIFO de entrada, DLQ e saída. Os serviços sobem
antes da API; workers, HTTP e conexões são gerenciados por Fx.

```powershell
docker compose --profile multi up --build -d --wait --wait-timeout 240
```

Esse modo inicia três APIs/workers independentes em `8080`, `8082` e `8083`,
compartilhando apenas PostgreSQL e SQS. Cada processo tem seu próprio pool de conexões.

## Autenticação

Não há cadastro de senhas nem emissão própria de tokens. Keycloak usa
`client_credentials`, assina tokens RS256 e inclui a audiência `financial-api`.

| Cliente | Secret local | Permissão |
|---|---|---|
| `internal-service` | `local-internal-secret` | Carteiras, ledger, reconciliação e métricas |
| `provider-a` | `local-provider-a-secret` | Transações de `provider-a` |
| `provider-b` | `local-provider-b-secret` | Transações de `provider-b` |

PowerShell:

```powershell
$tokenURL = 'http://localhost:8081/realms/jungle/protocol/openid-connect/token'
$internalToken = (Invoke-RestMethod -Method Post -Uri $tokenURL -Body @{
    grant_type = 'client_credentials'
    client_id = 'internal-service'
    client_secret = 'local-internal-secret'
}).access_token

$providerToken = (Invoke-RestMethod -Method Post -Uri $tokenURL -Body @{
    grant_type = 'client_credentials'
    client_id = 'provider-a'
    client_secret = 'local-provider-a-secret'
}).access_token

$internalHeaders = @{ Authorization = "Bearer $internalToken" }
$providerHeaders = @{
    Authorization = "Bearer $providerToken"
    'Idempotency-Key' = 'provider-a:bet-001'
}
```

## Abrir carteira e apostar

```powershell
$playerID = [guid]::NewGuid().ToString()
$walletBody = @{
    playerId = $playerID
    initialBalance = @{ amount = '100.00'; currency = 'BRL' }
} | ConvertTo-Json -Depth 5

$wallet = Invoke-RestMethod -Method Post -Uri 'http://localhost:8080/wallets' `
    -Headers $internalHeaders -ContentType 'application/json' -Body $walletBody

$betBody = @{
    providerId = 'provider-a'
    externalTransactionId = 'bet-001'
    playerId = $playerID
    walletId = $wallet.id
    roundId = 'round-001'
    gameId = 'game-001'
    kind = 'BET'
    money = @{ amount = '25.00'; currency = 'BRL' }
} | ConvertTo-Json -Depth 5

$result = Invoke-RestMethod -Method Post -Uri 'http://localhost:8080/wagering/transactions' `
    -Headers $providerHeaders -ContentType 'application/json' -Body $betBody
$result

# Repita a requisição idêntica: idempotentReplay será true, sem novo débito.
Invoke-RestMethod -Method Post -Uri 'http://localhost:8080/wagering/transactions' `
    -Headers $providerHeaders -ContentType 'application/json' -Body $betBody

Invoke-RestMethod "http://localhost:8080/wallets/$($wallet.id)" -Headers $internalHeaders
Invoke-RestMethod "http://localhost:8080/wallets/$($wallet.id)/ledger?limit=50" -Headers $internalHeaders
Invoke-RestMethod -Method Post "http://localhost:8080/wallets/$($wallet.id)/reconciliation" -Headers $internalHeaders
Invoke-RestMethod "http://localhost:8080/wagering/transactions/$($result.transactionId)" -Headers $providerHeaders
```

Uma abertura positiva cria `OPENING`, crédito no ledger e dois eventos de outbox no
mesmo commit. Abertura zero não cria movimentação nem eventos financeiros. O par
`(playerId,currency)` é único. Uma repetição da abertura retorna conflito.

## Contrato HTTP

| Rota | Identidade |
|---|---|
| `GET /health/live`, `GET /health/ready` | Pública |
| `POST /wallets`, `GET /wallets/{id}` | Interna |
| `GET /wallets/{id}/ledger?cursor=...&limit=50` | Interna |
| `POST /wallets/{id}/reconciliation` | Interna |
| `POST /wagering/transactions` | Provedor |
| `GET /wagering/transactions/{id}` | Provedor proprietário |
| `GET /providers/{providerId}/wagering/transactions/{externalId}` | Provedor proprietário |
| `GET /metrics` | Interna |

| HTTP | Situação | Corpo |
|---|---|---|
| 201 | Carteira criada | Carteira e versão |
| 200 | Operação processada ou replay | `transactionId,status,balance,idempotentReplay` |
| 202 | Referência ainda pendente | Resultado com `PENDING_REFERENCE` |
| 400 | Entrada inválida | `{"error":"INVALID_INPUT"}` |
| 401 | Token ausente, inválido ou expirado | `{"error":"UNAUTHENTICATED"}` |
| 403 | Permissão incompatível ou provedor divergente | `{"error":"FORBIDDEN"}` |
| 404 | Recurso inexistente ou transação de outro provedor | `{"error":"NOT_FOUND"}` |
| 409 | Duplicidade de carteira ou conflito de idempotência | `{"error":"CONFLICT"}` |
| 422 | Regra de negócio recusada | Resultado `REJECTED` com `failureCode` |
| 503 | Falha transitória de infraestrutura | `{"error":"TEMPORARILY_UNAVAILABLE"}`, `Retry-After: 2` |

Consultas de transação retornam 200 com o estado persistido, inclusive rejeições.
Valores são strings decimais com exatamente duas casas; escala excedente, sinais,
notação científica e negativos externos são rejeitados. BRL e USD são suportados,
sem conversão cambial. Zeros à esquerda são normalizados antes do hash.

## SQS e eventos

Filas criadas no startup:

- `wager-transactions.fifo`: entrada, visibility timeout de 60s, long polling de 10s.
- `wager-transactions-dlq.fifo`: redrive após cinco recebimentos sem conclusão.
- `wallet-events.fifo`: destino dos eventos da outbox.

No ambiente local, use o envelope dos requisitos acrescido de `accessToken`, contendo
um token OIDC do provedor. A identidade do token precisa coincidir com `data.providerId`.
Isso evita confiar em um campo de provedor fornecido pelo próprio remetente.

```json
{
  "messageId": "msg-001",
  "type": "WagerTransactionRequested",
  "occurredAt": "2026-10-23T12:00:00Z",
  "accessToken": "TOKEN_OIDC_DO_PROVEDOR",
  "data": {
    "providerId": "provider-a",
    "externalTransactionId": "bet-sqs-001",
    "idempotencyKey": "provider-a:bet-sqs-001",
    "playerId": "PLAYER_ID",
    "walletId": "WALLET_ID",
    "roundId": "round-001",
    "gameId": "game-001",
    "kind": "BET",
    "money": {"amount":"25.00","currency":"BRL"}
  }
}
```

Salve o envelope em `message.json` e envie com AWS CLI (valores locais `test/test`):

```powershell
$env:AWS_ACCESS_KEY_ID = 'test'
$env:AWS_SECRET_ACCESS_KEY = 'test'
$env:AWS_DEFAULT_REGION = 'us-east-1'
$queueURL = aws --endpoint-url http://localhost:4566 sqs get-queue-url `
    --queue-name wager-transactions.fifo --query QueueUrl --output text
aws --endpoint-url http://localhost:4566 sqs send-message --queue-url $queueURL `
    --message-body file://message.json --message-group-id WALLET_ID `
    --message-deduplication-id msg-001
```

Em AWS, a alternativa sem token no envelope exige configurar `SQS_PROVIDER_SENDERS`
como JSON mapeando `SenderId` do broker para o provedor. Exemplo:
`{"AIDA_IDENTIDADE_DO_PROVEDOR":"provider-a"}`. Nunca use um atributo de mensagem
controlado pelo remetente como identidade. Consulte o modelo IAM em `deploy/iam`.
O emulador local usa credenciais de desenvolvimento; a política IAM real não foi
validada em uma conta AWS nesta entrega.

Use `MessageGroupId=walletId` e um `MessageDeduplicationId` estável por envio lógico.
A deduplicação financeira não depende do FIFO: PostgreSQL e inbox continuam sendo
verificados em cada recebimento. Para testar reentrega, use dedup IDs diferentes
com o mesmo `messageId` e corpo. Mensagens inválidas não são apagadas; seguem retries
com backoff até a DLQ. Tokens vencidos na fila também exigem correção/reenvio.

Eventos de saída possuem `eventId,eventType,aggregateId,correlationId,occurredAt,version,data`.
Consumidores devem deduplicar por `eventId`: publicação é at-least-once. O destino
é uma fila, não um tópico de fan-out; integrações adicionais devem usar roteamento próprio.

## Testes

```powershell
go test ./...
go test -race ./...
go vet ./...
docker compose --profile multi up --build -d --wait --wait-timeout 240
go test -race -tags integration -count=1 -v ./tests/integration
```

A integração usa containers reais e três APIs. Exercita 50 apostas idênticas,
duas apostas de 80.00 sobre 100.00, isolamento de provedores, replay com saldo
original, LOSS, referência fora de ordem, dupla reversão, constraints, ledger,
reconciliação, SQS/inbox e publicação da outbox. O teste de recuperação interrompe
publicação após o envio e antes da confirmação; também encerra e reinicia as três
APIs com SIGKILL após commits, verificando pendências e idempotência persistentes.

Esses testes manipulam e reiniciam o ambiente local de desenvolvimento. Não os
aponte para um banco de produção. `TEST_DATABASE_URL` pode alterar a conexão da
suíte principal; a suíte de recuperação usa explicitamente o Compose local.

## Migrations

`migrations/001_initial.up.sql` é aplicada automaticamente uma vez, registrada em
`schema_migrations`. Um advisory lock de startup evita disputa entre migradores.
Esse lock não participa do processamento financeiro.

Para aplicar manualmente a migration inicial em ambiente vazio:

```powershell
Get-Content -Raw migrations/001_initial.up.sql | docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U financial -d financial
```

Para reverter, pare primeiro as APIs. **O comando abaixo remove os dados financeiros**:

```powershell
docker compose --profile multi stop api api2 api3
Get-Content -Raw migrations/001_initial.down.sql | docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U financial -d financial
```

O próximo startup reaplica a versão inicial. Em Linux, use `cat arquivo.sql | docker
compose exec -T postgres psql ...`. Em evolução futura, crie novas migrations;
não edite uma migration já aplicada.

## Configuração para executar Go fora do container

Exporte as variáveis de `.env.example` antes de `go run ./cmd/server`.
`DATABASE_URL`, `OIDC_ISSUER` e `OIDC_JWKS_URL` são obrigatórias. As demais:

| Variável | Padrão/finalidade |
|---|---|
| `HTTP_ADDRESS` | `:8080` |
| `OIDC_AUDIENCE` | `financial-api` |
| `AWS_REGION` | `us-east-1` |
| `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` | `test`, exclusivamente para execução local |
| `SQS_ENDPOINT` | `http://localhost:4566` |
| `SQS_QUEUE_URL`, `SQS_EVENTS_URL`, `SQS_DLQ_URL` | Se as três forem fornecidas, usa filas pré-provisionadas sem CreateQueue; caso contrário, provisiona o ambiente local |
| `SQS_PROVIDER_SENDERS` | Mapeamento opcional de identidades autenticadas pelo broker |

## Encerrar

```powershell
docker compose --profile multi down
```

O volume PostgreSQL preserva os dados. `down -v` apaga os volumes e deve ser usado
somente quando for intencional descartar o ambiente de desenvolvimento.
