# Revisão e correções do diff local contra `b956c29`

- Data: 2026-09-26.
- Base: `main` em `b956c29`; a revisão inicial incluiu 33 arquivos rastreados modificados e 25 novos, sem alterações staged.
- Estado: achados corrigidos no workspace local; nenhum commit, push ou deploy foi feito.
- Escopo atual: build local, API, dados, segurança, UI e concorrência do cache. O caminho de deploy remoto foi removido do projeto.

## Cobertura

Foram examinados os arquivos alterados e seus consumidores nos fluxos de autenticação, criação e edição de links, redirect, analytics, rate limiting, Redis, PostgreSQL, worker, UI, migrações, OpenAPI, Docker, Compose, CI e documentação. Os testes anteriores relatados no handoff da UI foram reexecutados. A revisão não é uma auditoria integral de dependências ou da infraestrutura externa.

## Achados e resolução

| ID | Prioridade | Condição e impacto | Resolução local |
| --- | --- | --- | --- |
| REV-001 | Alta | O antigo caminho de deploy anunciava URL HTTPS sem oferecer TLS na origem; links públicos poderiam falhar. | O caminho de deploy remoto, seus arquivos e as instruções correspondentes foram removidos. O alvo suportado nesta entrega é o Compose local. |
| REV-002 | Alta | O teste PostgreSQL aceitava `DATABASE_URL` genérico, aplicava migrações e gravava registros. | Passou a exigir `TEST_DATABASE_URL`, `TEST_DATABASE_ISOLATED=1` e banco com nome terminado em `_test`; foi executado em contêiner descartável. |
| REV-003 | Média | Um `CF-Connecting-IP` forjado, repassado por outro proxy confiável, alterava a chave do rate limit e o hash de analytics. | O header é ignorado; há teste de regressão para proxy confiável com header forjado. |
| REV-004 | Média | Edição/desativação podiam gravar no banco, falhar na invalidação Redis e responder `503`; uma leitura concorrente podia repopular o cache com dados antigos. | Um cache miss lê sob `SELECT ... FOR SHARE` até terminar o `SET`. Escritas mantêm o lock da linha, fazem `DEL` antes do commit e revertem a transação se o `DEL` falhar. Não há `SET` após criação ou escrita. Testes cobrem a intercalação entre instâncias, ordem de escritores e rollback. |
| REV-005 | Média | Ao selecionar outro link, uma falha de analytics deixava números do link anterior na tela. | A UI limpa os números na troca; regressão Playwright passou. |
| REV-006 | Média | O OpenAPI não representava `expires_at: ""`, usado pela UI para remover a validade. | Contrato e implementação agora descrevem o mesmo valor. |
| REV-007 | Alta | Segredos em `.env` podiam entrar no contexto e numa camada intermediária do build. | `.dockerignore` exclui variantes de `.env`; Dockerfile copia apenas fontes necessárias. Build local passou. |
| REV-008 | Baixa | O OpenAPI omitia `429` e `503` dos endpoints sujeitos a rate limit. | Respostas documentadas no contrato. |

Também foi corrigida a aceitação de limites de requisição zero ou negativos na configuração.

## Validação obtida

| Verificação | Resultado | Limite |
| --- | --- | --- |
| `go test ./...` | Passou. | Sem `TEST_DATABASE_URL`, os testes PostgreSQL destrutivos ficam protegidos e pulam. |
| `go test ./internal/repository/postgres -run 'TestInsertClickAndAnalytics|TestCachePopulationAndUpdateSerializeOnPostgresRow|TestCacheInvalidationFailureRollsBackUpdateAndDeactivate|TestConcurrentWritersInvalidateInCommitOrder' -count=1 -v` | Passou com PostgreSQL 16 descartável em banco `shortener_test`. | Contêiner local sem volume; não foram usadas credenciais de produção. |
| `go vet ./...` | Passou. | Cache Go no workspace. |
| `golangci-lint` v2.13.1 com configuração v2 temporária | Passou, 0 issues. | O arquivo de configuração do projeto é v1. |
| `golangci-lint` v1.64.8, versão fixada pelo CI | Passou. | Executado em imagem Docker local. |
| Testes de lógica da UI | 6 passaram; 85,14% de linhas em `logic.js`. | Testam funções isoladas. |
| Playwright | 5 passaram. | API simulada; não substitui teste completo com serviços reais. |
| `docker compose config` e `docker compose --profile obs config` | Passaram. | Validação de configuração local. |
| `docker build -t urlshortner-local-review .` | Passou. | Imagem apenas local. |
| `go build ./cmd/api` | Passou com código 0. | O Go avisou que não conseguiu escrever cache estatístico no módulo global por permissão do sandbox. |
| `gofmt -l cmd internal`, `git diff --check` | Sem pendências. | Revisão de formatação e whitespace. |

## Limites e risco residual

O protocolo de lock cobre a corrida entre instâncias que compartilham o mesmo PostgreSQL e Redis enquanto ambos operam normalmente. Uma restauração de snapshot antigo do Redis após reinício pode reintroduzir uma entrada obsoleta; isso exige política de persistência ou limpeza do cache na recuperação e não foi reproduzido nesta revisão. Os testes de navegador usam API simulada. Não foi feita publicação nem validação de HTTPS público.
