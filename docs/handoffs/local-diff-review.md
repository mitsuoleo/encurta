# Handoff — Revisão e correções locais

- Protocolo: HANDOFF v1
- Origem: revisão e implementação local
- Destino: manutenção local
- Estado: concluído
- Atualização: 2026-09-26
- Referência: `main`, diff do workspace contra `HEAD b956c29`, incluindo arquivos novos
- Fonte canônica: [relatório de revisão](../reviews/local-diff-b956c29.md)

## Objetivo e escopo

Revisar o diff local, corrigir os achados e a corrida de cache entre instâncias, remover o caminho de deploy remoto e entregar somente um build local. Nenhum commit, push ou deploy foi feito.

## Trabalho realizado

Os achados REV-001 a REV-008 estão resolvidos no escopo local. A resolução de cache usa lock compartilhado durante a leitura que preenche Redis e lock exclusivo para alterações, com invalidação antes do commit e rollback se Redis falhar. O Compose local e o perfil de observabilidade permanecem disponíveis.

## Contratos e decisões

O OpenAPI documenta `expires_at: ""` para limpar a validade e `429`/`503` em endpoints limitados. O teste PostgreSQL exige `TEST_DATABASE_URL`, `TEST_DATABASE_ISOLATED=1` e banco `_test`. O cache é preenchido apenas em miss de redirect, dentro da transação de leitura. A restauração de snapshot Redis antigo após reinício exige tratamento operacional separado.

## Validação

| Verificação | Resultado |
| --- | --- |
| `go test ./...` | Passou; integração destrutiva ficou protegida e pulou sem `TEST_DATABASE_URL`. |
| Testes PostgreSQL destrutivos em PostgreSQL 16 descartável | Passaram, incluindo locks de cache, rollback e dois writers. |
| `go vet ./...`, lint v2 instalado e lint v1 do CI | Passaram. |
| Testes de lógica da UI e Playwright | 6 e 5 passaram, respectivamente. |
| `go build ./cmd/api`, Compose local, perfil `obs` e build Docker local | Passaram. O `go build` emitiu aviso de cache global bloqueado pela sandbox, mas saiu com código 0. |
| Formatação Go e `git diff --check` | Sem pendências. |

## Próxima ação

Revisar o diff local antes de qualquer commit. Não há etapa de deploy nesta entrega.

## Limites e recuperação

Os testes Playwright usam API simulada. O protocolo de cache não cobre a restauração de dados antigos por persistência Redis após reinício. As mudanças continuam somente no workspace e não tocaram produção.
