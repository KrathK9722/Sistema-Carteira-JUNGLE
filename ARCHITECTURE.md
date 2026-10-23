# Decisões e limites da implementação

## Estrutura e transação SQL

`domain` não depende de Fx, HTTP, SQS ou pgx. `Money`, `Wallet`, a máquina de estados,
as regras de referência e o construtor do ledger mantêm as invariantes em memória.
Criação e reidratação são distintas; reidratar preserva saldo, datas e versão e não
emite eventos. Entradas externas são comandos normalizados, não entidades mutáveis.

`application.Service` oferece o caso de uso compartilhado por HTTP e SQS, faz a
normalização e depende da interface `Store`. `postgres.Repository` implementa esse
contrato e delimita a transação entre as tabelas. Por simplicidade, a orquestração
atômica está concentrada nesse adaptador: ele chama os métodos de domínio enquanto
mantém o lock SQL. Não há commits independentes de repositórios de saldo e ledger.

pgx com SQL explícito foi escolhido para deixar locks, statements e commit visíveis.
Todo I/O recebe contexto. HTTP tem prazo de 15s; trabalho de mensagem tem 20s,
menor que a visibilidade de 60s. Falha antes do commit causa rollback; resposta ou
ack perdido depois do commit é resolvido pela idempotência persistente.

## Dinheiro

`int64` em unidades mínimas, duas casas, BRL/USD. `BIGINT` preserva exatamente o
valor; moeda é persistida junto. Limites: -92233720368547758.08 a
92233720368547758.07 em cálculos internos. Entradas financeiras não aceitam negativos,
float, expoente, espaços, escala diferente de duas casas ou arredondamento implícito.
Zeros à esquerda são aceitos e normalizados antes do hash. JSON usa strings.

## Concorrência e invariantes no banco

`SELECT ... FOR UPDATE` na carteira serializa apenas operações daquela carteira.
Instâncias e carteiras diferentes usam conexões independentes; não há mutex global.
O advisory lock é usado exclusivamente para migrations no startup.

Schema: saldo >= 0, versão >= 1, unicidade de jogador/moeda, identidade externa e
idempotência. Ledger tem unicidade carteira/transação e carteira/versão, equação
financeira com `NUMERIC` no CHECK e trigger que confere a operação processada e a
cadeia de saldos. Constraints diferidas comparam saldo e versão da carteira com o
ledger ao commit. Portanto, atualizar apenas o saldo sem ledger é recusado.
Triggers proíbem UPDATE, DELETE e TRUNCATE do ledger e alteração/remoção de
transações terminais. Não se pretende proteger contra um administrador que desative
triggers ou remova o schema; produção deve separar proprietário/migrador do usuário
de aplicação e restringir privilégios.

## Abertura e ledger

Abertura positiva confirma carteira versão 1, operação interna `OPENING/PROCESSED`,
ledger CREDIT e eventos `WagerTransactionProcessed` e `WalletBalanceChanged` juntos.
Schema distingue origem interna dos campos externos inaplicáveis. Abertura zero
salva somente carteira. Não se usa `Wallet.Credit` para reaplicar o saldo inicial.
O ledger é um registro simples de movimentos, não partidas dobradas.

## Idempotência

Chaves únicas `(provider_id,idempotency_key)` e `(provider_id,external_id)` sobrevivem
à reinicialização. Payload equivalente com a mesma chave retorna o resultado
original persistido, incluindo o saldo observado naquele processamento. Reutilizar
uma chave com outro conteúdo ou a identidade externa com outra chave retorna 409.

Hash SHA-256 sobre JSON de chaves ordenadas de: providerId, externalTransactionId,
playerId, walletId, roundId, gameId, kind, money.amount, money.currency e
referenceExternalTransactionId (vazio quando ausente). Valores decimais são
normalizados antes do hash. Chave de idempotência, token e metadados de transporte
não fazem parte desse hash. Os mesmos comandos são usados por HTTP e SQS.

Inbox: chave `(consumer_name,message_id)`, hash do envelope reserializado sem o token
e conclusão no mesmo commit da operação, ledger e outbox. Uma pendência de referência
persistida é conclusão durável de entrada; sua continuidade pertence ao worker.

## Estados e referências

`PENDING -> PROCESSED | REJECTED | PENDING_REFERENCE | FAILED`.
`PENDING_REFERENCE -> PROCESSED | REJECTED | FAILED | PENDING_REFERENCE`.
Estados terminais não aceitam novas transições. Não há commit intermediário de
PENDING: a operação termina ou vira pendência de referência na mesma transação.

Referências são resolvidas por provedor/ID externo. Conferem jogador, carteira,
moeda e rodada. REFUND exige BET; ROLLBACK exige BET/WIN/REFUND e valor integral.
WIN pode referenciar BET da mesma rodada. LOSS exige zero, não muda saldo/versão e
gera somente o evento de processamento. OPENING externo é inválido.

Política conservadora de reversões: uma operação pode ter **uma única reversão direta
bem-sucedida**, independentemente de ser REFUND ou ROLLBACK. Assim, não se devolve
duas vezes o mesmo débito por tipos diferentes. ROLLBACK de REFUND reverte o crédito
do REFUND, mas não libera o BET original para outro refund. ROLLBACK de ROLLBACK
não é aceito. Reversão sem saldo é rejeitada com código distinto de BET sem saldo.

Referência ausente ou ainda pendente: persistência de PENDING_REFERENCE, backoff
exponencial até 300s, máximo 30 tentativas ou TTL de 24h. Worker retoma após restart.
Referência terminal sem sucesso rejeita imediatamente. Códigos estáveis:

- `INSUFFICIENT_BALANCE`: BET sem saldo.
- `REVERSAL_INSUFFICIENT_BALANCE`: ROLLBACK que não pode debitar.
- `AMOUNT_OUT_OF_RANGE`: operação excederia o limite numérico.
- `REFERENCE_NOT_FOUND`: tentativas/TTL esgotados.
- `REFERENCE_NOT_PROCESSED`: referência rejeitada ou permanentemente falha.
- `REFERENCE_MISMATCH`: vínculo de provedor/jogador/carteira/moeda/rodada incompatível.
- `REFERENCE_AMOUNT_MISMATCH`: tentativa de reversão parcial ou valor diferente.
- `INVALID_REFERENCE_KIND`: referência de tipo inadequado.
- `ALREADY_REVERSED`: já existe reversão bem-sucedida.

Entradas inválidas e conflitos são corrigíveis pelo cliente, sem movimento.
Rejeições persistidas são definitivas para aquela identidade; replay não tenta
novamente, mesmo após crédito posterior. Falhas de I/O causam rollback/retry, não
rejeição financeira. FAILED existe na máquina de estados, mas não há mecanismo de
classificação automática de falha permanente de infraestrutura nesta versão.

## Autenticação e autorização

Keycloak é o IdP externo. Verificação RS256, assinatura por JWKS, issuer, audiência
e expiração. Claims de autorização são emitidos por mappers configurados no IdP,
nunca inferidos do corpo. Carteiras são internas; provedor só envia e consulta suas
próprias transações. Consultas por ID interno também filtram providerId. Replays
passam pelas mesmas verificações antes de consultar dados financeiros.

SQS local usa token OIDC no envelope. Alternativamente, em broker IAM protegido,
`SQS_PROVIDER_SENDERS` vincula o SenderId de sistema ao provedor. O atributo de
sistema é fornecido pelo SQS, não por atributos arbitrários no corpo; referência:
https://docs.aws.amazon.com/AWSSimpleQueueService/latest/APIReference/API_ReceiveMessage.html
As políticas IAM de exemplo restringem produtor a SendMessage e consumidor às
ações necessárias. Elas exigem provisionamento de identidades/ARNs reais e não
foram validadas em AWS. LocalStack não é apresentado como prova de enforcement IAM
de produção. Credenciais locais compartilhadas são uma limitação do ambiente.

## Inbox, outbox e entrega

Eventos são snapshots tipados com versão 1 e IDs aleatórios estáveis. A outbox só
é visível após o commit. Publishers usam `FOR UPDATE SKIP LOCKED`, mantendo o lock
durante o envio e confirmação; não há lease local que possa ficar abandonado.
Conexão/processo encerrado libera o lock e outra instância retenta. Esse desenho
segura uma conexão durante a chamada de rede; timeout e volume limitam o custo.
Backoff exponencial máximo 300s, tentativas persistidas. Publicação aceita pelo SQS
seguida de interrupção antes do commit pode ser repetida com o mesmo eventId.
Consumidores externos devem deduplicar esse ID. Não há promessa de exactly-once.

FIFO é otimização de ordenação por walletId. A identidade financeira não depende
da janela de deduplicação do broker. DeleteMessage ocorre somente após commit.
Mensagens inválidas, tokens inválidos e erros transitórios ficam sem ack; backoff
de visibilidade 2,4,8,16,32s até redrive em cinco recebimentos. Em shutdown,
cancelamento interrompe polling, processamento abortado faz rollback e sua
visibilidade é liberada; processamento já confirmado pode ser entregue novamente
sem novo movimento. Tokens não são persistidos nem registrados em logs.

## Reconciliação e observabilidade

Uma única consulta SQL usa uma visão MVCC consistente para saldo e soma do ledger,
incluindo abertura. Soma e diferença usam NUMERIC no banco para evitar overflow do
acumulador; resposta decimal exata sem converter para float. Não altera saldo.
Divergências aparecem na resposta, log JSON e métrica. Logs financeiros contêm
identificadores e status, não payloads completos. Há métricas HTTP básicas e health
checks de PostgreSQL/SQS. Métricas avançadas de DLQ, latência e atraso da outbox
ainda são uma pendência, assim como tracing opcional.

## Fx e testes

Construtores fornecem configuração, pool, adaptadores, caso de uso, autenticação,
handlers e workers via fx.Module/fx.Provide. fx.Invoke registra lifecycle.
Startup valida configuração, conecta, migra, cria filas e abre HTTP. Shutdown em
ordem inversa para HTTP, aguarda/cancela workers e só então fecha o pool.

GitHub Actions roda testes unitários com race detector, vet, build Compose e testes
com PostgreSQL, Keycloak, SQS e três processos reais. A integração não substitui a
infraestrutura inteira por mocks. O teste unitário de verificação JWT usa JWKS de
teste adicionalmente à integração real com o Keycloak.

## Limitações e trabalho restante

- Credenciais locais/IAM real e TLS de produção não estão prontos para deploy público.
- Envelope SQS local exige token adicional; consumidores que usam exatamente o
  envelope original precisam do mapeamento SenderId e broker IAM configurados.
- Métricas cobrem apenas parte da lista solicitada; não há dashboards/tracing.
- Falhas transitórias são retentadas, mas não classificadas automaticamente como
  FAILED permanente no banco; o broker pode encaminhá-las à DLQ.
- Testes não esgotam todas as combinações de referências/reversões, todas as janelas
  de crash do consumidor, indisponibilidade prolongada de infraestrutura ou teste
  de carga. Não há alegação de cumprimento integral do desafio.
- O Docker local do computador de desenvolvimento não inicializou; integração
  real é executada pelo runner Linux do GitHub Actions.
