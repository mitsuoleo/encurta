# Estado da documentação e lacunas

Esta entrega foi investigada a partir da revisão Git `b956c29` e das alterações locais presentes em 26/09/2026. Os caminhos e símbolos abaixo permitem conferir as afirmações. “Observado no código” não significa “verificado em execução”. Handoffs e revisões locais foram usados para contexto, sem tratá-los como prova de produção.

| Item | Área | Tipo e evidência | Impacto | Próxima ação |
| --- | --- | --- | --- | --- |
| Implantação pública | Operação | Ausente no estado atual; arquivos de deploy remoto removidos, Compose local em `docker-compose.yml` | Não há procedimento apoiado para operar um domínio público ou HTTPS | Definir arquitetura, segurança e operação antes de documentar publicação |
| Backup, restauração e rollback | Dados e operação | Não verificado: migrações em `migrations/`, volume `pgdata` no Compose, sem runbook presente | Não há caminho comprovado para recuperar dados ou reverter uma atualização | Criar e testar procedimentos em ambiente isolado antes de adotá-los |
| Validação de URL por DNS | Segurança | Ausente em `internal/domain/url.go`; validação cobre nomes conhecidos e IPs literais | Nome público pode resolver para rede interna sem ser detectado na criação | Decidir política de destinos caso o serviço seja publicado |
| Revogação de token | Autenticação | Ausente: `internal/handler/http.go` devolve `204` no logout e `internal/web/app.js` remove token local | Token capturado ainda pode valer até expirar | Definir necessidade de revogação antes de uso público |
| Perda de clique quando `XADD` falha | Analytics | Observado em `internal/worker/clicks.go` e `internal/handler/http.go` | Contagem pode ser menor que redirects efetivos | Definir requisito de confiabilidade e mecanismo correspondente, se necessário |
| Teste com API e serviços reais | Validação | Não executado nesta entrega; `scripts/smoke.ps1` requer Compose local | Fluxo completo não foi confirmado nesta execução | Executar em ambiente isolado autorizado e registrar resultado |
| Responsável e periodicidade documental | Manutenção | Informação não fornecida; sem definição localizada no repositório | Revisões podem depender de iniciativa individual | Definir responsável e gatilho de revisão no processo do projeto |
| Nome local dos recursos Compose | Operação | `docker-compose.yml` não define `name`; Compose deriva o projeto do nome da pasta atual | Rede e volume existentes podem conservar um prefixo anterior mesmo com o repositório renomeado | Planejar migração do volume antes de fixar outro nome de projeto, para não apresentar um banco vazio como se fossem os dados existentes |

## Verificações desta entrega

| Verificação | Resultado | Limite |
| --- | --- | --- |
| `go test ./...` com `GOPROXY=off` | Passou após acesso autorizado aos caches locais do Go | Testes PostgreSQL que exigem `TEST_DATABASE_URL` e `TEST_DATABASE_ISOLATED=1` não rodaram |
| `go test ./...` e `go vet ./...` após renomear o módulo para `github.com/mitsuoleo/encurta` | Passaram; `go list -m` devolveu o novo caminho | Integração PostgreSQL continua sem opt-in |
| `go test ./internal/handler` após editar OpenAPI | Passou | Testes HTTP não substituem Compose real |
| Primeira tentativa de `go test ./...` no sandbox padrão | Falhou por permissão de escrita nos caches Go do perfil do usuário | Falha de ambiente, não de teste; a repetição acima passou |
| `node --test --experimental-test-coverage internal/web/logic.test.cjs` | Passou: 6 testes, 85,14% das linhas de `logic.js` | Não testa browser nem API real |
| `npm run test:ui` | Passou: 5 fluxos no Chrome | Usa API simulada, sem PostgreSQL ou Redis reais |
| Capturas `docs/ui-login-encurta.png` e `docs/ui-dashboard.png` | Geradas e inspecionadas visualmente da UI atual no Chrome | Dados do painel são fictícios e vêm de uma API simulada temporária |
| JSON do dashboard Grafana e consultas de métricas | Parse passou; cinco referências conferidas com os nomes `encurta_` definidos no código | Não houve scrape de Prometheus nem execução de Grafana |
| Parse de `internal/web/openapi.yaml` com `gopkg.in/yaml.v3` | Passou; 21 referências locais resolvidas após corrigir uma descrição YAML preexistente com dois-pontos sem aspas | Verifica sintaxe e referências locais, não todas as regras semânticas OpenAPI |
| Verificação local de links e âncoras Markdown | Passou: 21 arquivos, nenhum destino ausente | Não consulta URLs externas nem renderiza Mermaid |
| `git diff --cached --check` antes do commit | Passou após corrigir dois detalhes de whitespace; incluiu os arquivos novos preparados no índice | Verifica espaços e marcadores do diff, não formatação editorial |
| `npm audit --offline --audit-level=high` | Passou: nenhuma vulnerabilidade encontrada nas informações disponíveis localmente | Sem consulta ao registro remoto de advisories |
| Espaços finais nos arquivos novos e editados desta entrega | Passou: 13 arquivos sem achados | Um espaço final em handoff preexistente foi preservado, fora do escopo |
| Compose, smoke e serviços externos | Não executados | Fora da validação sem serviços persistentes autorizada para esta entrega |

Inspeção estática de código, scripts e testes sustenta descrições de implementação, mas não equivale a executar os fluxos. A documentação não assume acesso a produção.

## Como manter

Em cada alteração, confira: contrato OpenAPI e exemplos quando mudar HTTP; guia e apresentação quando mudar funcionalidade; instalação e operação quando mudar configuração, schema ou dependência; arquitetura e ADRs quando mudar decisão estrutural. Revise links, comandos e afirmações de status antes de publicar a documentação. Não há tradução adicional a sincronizar nesta entrega.
