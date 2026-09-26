# Capacidades e limites atuais

Este documento descreve o estado do código local, inclusive alterações não commitadas, sobre a revisão `b956c29`. A [apresentação](../README.md) resume o produto; o [guia de uso](../user-guide/using.md) mostra as tarefas no painel.

## O que está implementado

- Conta por e-mail e senha; cadastro e login emitem um token de 24 horas. A gestão de links exige esse token; abrir um link curto é público. Evidência: `internal/service/auth.go`, `internal/auth/token.go`, `internal/handler/http.go`.
- Criação de links com código aleatório de sete caracteres ou alias de 3 a 20 caracteres. Alias duplicado é rejeitado. Pode-se definir uma data futura de validade. Evidência: `internal/domain/code.go`, `internal/service/link.go`.
- Lista paginada dos links do dono, edição do destino e da validade, desativação e reativação. Um link inativo ou vencido responde `410` ao ser aberto. Evidência: `internal/service/link.go`, `internal/domain/link.go`, `internal/handler/http.go`.
- Painel com cópia do link, código QR gerado localmente, temas claro/escuro e analytics do dono. A interface é embutida no binário Go. Evidência: `internal/web/index.html`, `internal/web/app.js`, `internal/web/embed.go`.
- Contagens de cliques, visitantes únicos por hash de IP, dias, referências, dispositivos e navegadores. O redirect publica um evento no Redis Stream; um worker grava o clique no PostgreSQL. Evidência: `internal/domain/link.go`, `internal/handler/http.go`, `internal/worker/clicks.go`, `internal/repository/postgres/store.go`.

## Regras que afetam o uso

O destino deve usar `http://` ou `https://`. O código rejeita credenciais embutidas na URL, `localhost`, hosts `.local` e endereços IP privados ou locais escritos diretamente, inclusive algumas formas IPv4 alternativas. Ele não resolve o DNS de um nome durante a criação; portanto, isso não equivale a uma verificação de reputação ou a uma defesa completa contra nomes que apontem para redes internas. Evidência: `internal/domain/url.go`.

Um alias aceita letras, números e hífen, sem hífen no início ou fim, sem hífen duplo e sem nomes reservados. A validade precisa estar no futuro. Remover a validade de um link existente é possível na edição. Evidência: `internal/domain/code.go`, `internal/service/link.go`, `internal/handler/http.go`.

O registro de analytics é assíncrono: um clique pode aparecer depois do redirecionamento. Se a publicação no Redis falhar, o redirect ainda acontece e aquele clique pode não ser registrado. Sair da interface apaga o token local; não há revogação imediata no servidor. Evidência: `internal/worker/clicks.go`, `internal/handler/http.go`, `internal/web/app.js`.

## Estágio e operação

O alvo suportado neste repositório é o Docker Compose **local**. O perfil opcional de observabilidade inclui Prometheus e Grafana locais. Arquivos anteriores de deploy remoto foram removidos do diretório de trabalho; não há procedimento vigente e verificado para publicar o serviço, fazer backup ou restaurar dados. A [operação local](../operations/local.md) descreve apenas o que a configuração presente permite conferir.
