# Segurança no código atual

Esta página descreve mecanismos observados no código local, sem afirmar que houve auditoria ou implantação segura em produção. Evidências principais: `internal/auth/`, `internal/middleware/`, `internal/domain/url.go`, `internal/config/config.go` e testes correspondentes.

## Identidade e acesso

Senhas são transformadas com bcrypt antes de entrar em `users`. O cadastro exige 8 a 72 bytes; e-mails são normalizados para minúsculas. JWTs HS256 duram 24 horas e protegem `/links`. O serviço confere a propriedade antes de editar, desativar ou consultar analytics, e responde `404` para link alheio. O redirect não exige autenticação. A UI guarda o token em `localStorage`; **Sair** o apaga localmente, sem revogação no servidor.

## Entradas e limites

O servidor aceita destinos HTTP(S) e rejeita credenciais na URL, hosts locais conhecidos e IPs privados/locais literais. Não há resolução DNS no momento da criação nem checagem de reputação; nomes que mais tarde resolvam para redes internas não são bloqueados por essa validação. Alias, validade e payload JSON também são validados. A API aplica limites Redis a autenticação, criação e redirects. Se a verificação de limite falhar, autenticação e criação retornam `503`, enquanto o redirect prossegue (`internal/middleware/ratelimit.go`).

O IP para analytics vira SHA-256 de sal + IP; o valor puro não é armazenado na tabela de cliques. O referenciador perde query e fragmento quando é uma URL analisável, e metadados de clique são limitados em comprimento (`internal/domain/clickmeta.go`). O hash ainda é um identificador derivado, e o referenciador remanescente pode conter informação; não se deve tratar analytics como anonimização completa.

## Respostas e configuração

Os headers incluem `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, política de referência e CSP sem script inline (`internal/middleware/headers.go`). `/metrics` exige um bearer estático quando `METRICS_TOKEN` está definido; sem ele, é público. O Compose local deixa esse token vazio. `ENV=production` exige segredos próprios, token de métricas e base pública HTTPS, mas não fornece TLS, proxy ou deploy remoto por si só. Consulte [configuração](../reference/configuration.md).

Antes de publicar o serviço, faltam decisões e validações de implantação, gestão de segredos, rede e recuperação registradas em [lacunas](../maintenance/documentation-status.md). Não há promessa de proteção além dos mecanismos descritos acima.
