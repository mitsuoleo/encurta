# Handoff — Reformulação da interface

- Protocolo: HANDOFF v1
- Origem: Frontend
- Destino: revisão do produto e integração
- Estado: pronto para revisão
- Atualização: 2026-09-26
- Referência do trabalho: branch `main`, alterações locais não commitadas; preservar as demais modificações preexistentes
- Fontes canônicas: [README](../../README.md), [contrato OpenAPI](../../internal/web/openapi.yaml), [rotas HTTP](../../internal/handler/http.go), [interface](../../internal/web/index.html)

## Objetivo

Oferecer uma jornada diária clara e acessível para cadastro, login, criação, gestão e analytics de links, com temas claro e escuro e layout móvel.

## Escopo

Incluído: UI em `/`, assets embutidos em `/ui/`, mensagens e estados de falha, testes de lógica e navegador.
Excluído: novos endpoints de negócio, mudança de autenticação, publicação e migração de dados.

## Trabalho realizado

- Implementado: novo painel responsivo, temas com preferência do sistema e escolha persistida, criação com QR e cópia, lista paginada, detalhes, edição, confirmação de desativação, reativação e analytics.
- Implementado: CSS, lógica, interface e QR em assets separados; CSP sem `unsafe-inline`; testes HTTP dos assets, testes unitários da lógica e testes de navegador com API simulada.
- Verificado no serviço local real: cadastro, criação com alias, edição, desativação, reativação, redirect 302 para o destino editado e atualização dos analytics após um clique.

## Decisões e contratos

- Contratos de negócio mantidos: `/auth/*`, `/links`, `/links/{code}` e `/links/{code}/analytics`; nenhum endpoint de negócio novo.
- `/ui/app.css`, `/ui/logic.js`, `/ui/qr.js` e `/ui/app.js` são assets públicos embutidos. O QR continua local, sem serviço externo.
- O token permanece no `localStorage`, conforme implementação anterior. Um `401` de recurso autenticado encerra a sessão local e solicita novo login.
- Links inativos com validade vencida pedem ajuste ou remoção da validade antes de reativar.

## Validação

| Verificação | Ambiente/condição | Resultado | Evidência ou motivo |
| --- | --- | --- | --- |
| `go test ./...` | Windows, Go 1.27.1 | passou | todos os pacotes executados |
| `go vet ./...`, `gofmt -l`, `git diff --check` | Windows | passou | sem diagnóstico ou arquivo pendente |
| Build da API | Docker Compose, Go 1.23 na imagem | passou | `docker compose up --build -d` |
| Lógica da UI | Node 26 | passou | 6 testes; 85,14% de linhas em `logic.js` |
| Navegador automatizado | Chrome local, API simulada | passou | 4 testes com `npm run test:ui` |
| Jornada com API real | Chrome e Compose local | passou | cadastro, criação, edição, estado, redirect e analytics |
| Responsividade e tema | Chrome, 320 px e 375 px | passou | sem rolagem horizontal; tema persistiu após recarga |
| `golangci-lint` v1.64.8 | Imagem Docker usada pelo CI | passou | `golangci-lint run ./...`, sem diagnósticos |

## Pendências e dependências

- A suíte de navegador usa respostas simuladas; a integração real foi verificada manualmente no Compose local. Não há job novo de navegador no CI.
- A cobertura total do pacote `internal/handler` medida localmente foi 71,8%; os testes acrescentados cobrem as rotas de assets, enquanto a meta de 80% foi atingida na lógica nova da UI.
- O teste real criou uma conta e um link locais no volume de desenvolvimento. Não foi feita limpeza destrutiva do banco.

## Próxima ação

Se o produto for publicado, validar a aparência e os fluxos com o domínio e a configuração HTTPS definitivos.

## Critérios de aceite

Entrar, criar e editar um link, desativá-lo e reativá-lo, consultar analytics e alternar os temas devem funcionar em celular e desktop; erros devem indicar recuperação sem perder entradas úteis.

## Riscos e recuperação

O novo servidor de assets e a CSP devem ser publicados juntos; retirar apenas uma parte impede a UI de carregar. A versão anterior pode ser restaurada pelo controle de versão sem migração de dados.

## Histórico de passagens

- 2026-09-26: frontend concluiu a implementação e encaminhou o estado local para revisão.
