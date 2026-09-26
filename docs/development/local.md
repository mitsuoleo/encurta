# Desenvolver no ambiente local

## Preparar e iniciar

O serviço usa Go 1.23, PostgreSQL 16 e Redis 7 no Compose. A UI é HTML, CSS e JavaScript embutidos no binário; Node é usado apenas pelos testes da interface. A referência de versões está em `go.mod`, `Dockerfile`, `docker-compose.yml` e `package.json`.

No PowerShell, a partir da raiz do repositório:

```powershell
.\make.ps1 up
```

Isso executa `docker compose up --build -d`. Abra `http://localhost:8080`. Para parar sem remover o volume do PostgreSQL, use `.\make.ps1 down`; para acompanhar a API, `.\make.ps1 logs`. Em macOS/Linux com Make, os alvos equivalentes são `make up`, `make down` e `make logs`. O Compose publica a API na porta 8080; PostgreSQL e Redis ficam apenas na rede do Compose. O [guia de operação](../operations/local.md) cobre saúde e falhas.

O processo Go aplica as migrações de `migrations/` **ao iniciar**, antes de abrir a API, e falha se PostgreSQL ou Redis não estiverem prontos. `cmd/api/main.go` é a entrada; `internal/handler/` recebe HTTP, `internal/service/` contém regras, `internal/repository/` implementa armazenamento e `internal/worker/` grava cliques. Veja a [arquitetura](../architecture/overview.md).

## Configuração

O Compose já fornece valores de desenvolvimento. `.env.example` mostra substituições locais para `JWT_SECRET` e `IP_HASH_SALT`; não registre `.env` no Git. Consulte a [referência de configuração](../reference/configuration.md) antes de alterar variáveis. `ENV=production` aciona verificações adicionais de segredo e URL pública, mas este repositório não traz um procedimento de publicação.

## Verificar alterações

Inspecione o efeito de cada comando antes de executá-lo em outro ambiente. Os comandos abaixo são para a raiz do repositório:

```powershell
go test ./...
go vet ./...
node --test --experimental-test-coverage internal/web/logic.test.cjs
npm ci
npm run test:ui
```

`go test ./...` executa testes Go; o teste de integração PostgreSQL só roda com `TEST_DATABASE_URL` apontando para banco isolado com nome terminado em `_test` **e** `TEST_DATABASE_ISOLATED=1`. Sem essas condições, esse teste pula. Os testes Playwright usam Chrome e uma API simulada iniciada pelo próprio teste; não comprovam o comportamento com PostgreSQL e Redis reais. `npm ci` instala apenas dependências locais de teste. O CI atual executa testes Go e `golangci-lint` com serviços isolados; não executa Playwright (`.github/workflows/ci.yml`).

Há um script `scripts/smoke.ps1` destinado ao Compose local. Ele verifica saúde, cadastro, criação, redirect e analytics com dados fictícios. Como usa os serviços do Compose e grava registros locais, não foi executado nesta entrega. Consulte o [registro de verificações](../maintenance/documentation-status.md).

## Antes de um pull request

- Atualize guia e apresentação quando mudar uma funcionalidade; atualize [OpenAPI](../../internal/web/openapi.yaml) e exemplos quando mudar o contrato HTTP.
- Atualize [configuração](../reference/configuration.md), dados e [operação](../operations/local.md) quando mudar variável, migração ou dependência.
- Reavalie a [arquitetura](../architecture/overview.md) e ADRs se a estrutura mudar.
- Confira links, exemplos, testes afetados e o diff; registre o que não conseguiu verificar.

Não há responsável ou periodicidade de revisão documental definidos no repositório.
