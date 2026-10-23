# Arquitetura em desenvolvimento

Esta estrutura inicial ainda nao implementa as regras do desafio.

Decisoes previstas: dinheiro em centavos com int64; PostgreSQL com pgx;
bloqueio por carteira; idempotencia persistente; ledger imutavel; inbox e
outbox transacionais; Keycloak para identidade; SQS para mensageria;
Uber Fx para composicao e ciclo de vida.

Pendente: implementacao financeira, migracoes, autenticacao, autorizacao,
provisionamento das filas e identidades, workers, testes e uso efetivo de Fx.
