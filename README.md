# Servico financeiro - estrutura inicial

Este projeto contem somente a estrutura de pastas, um servidor de exemplo
e a configuracao inicial das dependencias. Nao e a solucao completa do desafio.

## Preparacao realizada neste computador

Go, Docker Desktop e WSL foram instalados. As bibliotecas de Uber Fx,
PostgreSQL, SQS e validacao OIDC foram baixadas e registradas em go.mod/go.sum.
O Visual Studio Code ja estava instalado.

O Docker nao iniciou: seus registros informam ausencia de virtualizacao,
e o Windows reportou VirtualizationFirmwareEnabled=False. Verifique a
virtualizacao no Gerenciador de Tarefas, em Desempenho > CPU. Se estiver
desabilitada, sera necessario habilita-la nas configuracoes de firmware
do computador. O procedimento depende do fabricante. Nenhuma reinicializacao
foi realizada automaticamente. As imagens dos servicos ainda nao foram baixadas.

Feche e reabra o terminal para reconhecer os programas instalados.

## Iniciar dependencias

Abra o Docker Desktop e aguarde o mecanismo iniciar. No PowerShell nesta pasta:

```powershell
docker compose up -d
docker compose ps
```

O primeiro comando baixa as imagens e inicia PostgreSQL, Keycloak e LocalStack.

## Iniciar programa Go

```powershell
go run ./cmd/server
```

Abra http://localhost:8080/health/live no navegador.
A resposta esperada e {"status":"running"}. Para encerrar, pressione Ctrl+C.

O servidor ainda nao se conecta aos servicos e nao carrega o arquivo .env.

## Enderecos locais

- Servidor Go: http://localhost:8080/health/live
- Keycloak: http://localhost:8081 (admin / local_admin_password)
- LocalStack: http://localhost:4566/_localstack/health
- PostgreSQL: localhost:5432, banco financial, usuario financial,
  senha local_development_password.

As credenciais sao exemplos locais. O Keycloak ainda precisa de configuracao
de identidades e clientes. As filas ainda precisam ser criadas.

## Verificar e parar

```powershell
go test ./...
go vet ./...
docker compose logs
docker compose down
```

Ainda nao existem testes de comportamento. O volume preserva os dados do banco.
Consulte ARCHITECTURE.md para os requisitos ainda pendentes.
