# Configuração

`internal/config/config.go` lê as variáveis abaixo; `docker-compose.yml` fornece valores para o ambiente local. Um valor padrão observado é aplicado quando a variável está vazia. Valores ilustrativos com segredo são fictícios e **não** servem para produção.

| Variável | Propósito | Obrigatória | Padrão no código | Exemplo fictício | Sensível | Origem |
| --- | --- | --- | --- | --- | --- | --- |
| `ENV` | Liga validações de produção quando igual a `production` | Não | `development` | `development` | Não | `config.Load` |
| `HTTP_ADDR` | Endereço de escuta HTTP | Não | `:8080` | `:8080` | Não | `config.Load` |
| `DATABASE_URL` | Conexão PostgreSQL | Há padrão local; conexão funcional é necessária | `postgres://shortener:shortener@localhost:5432/shortener?sslmode=disable` | `postgres://usuario:senha-ficticia@localhost:5432/shortener?sslmode=disable` | Sim | `config.Load`, Compose |
| `REDIS_URL` | Conexão Redis para cache, limites e stream | Há padrão local; conexão funcional é necessária | `redis://localhost:6379/0` | `redis://localhost:6379/0` | Pode ser | `config.Load`, Compose |
| `PUBLIC_BASE_URL` | Base retornada em `short_url` | Não; HTTPS e host não local quando `production` | `http://localhost:8080` | `http://localhost:8080` | Não | `config.Load` |
| `JWT_SECRET` | Assinatura dos tokens | Valor próprio exigido quando `production` | valor de desenvolvimento em `config.go` | segredo longo fornecido fora do Git | Sim | `config.Load`, `.env.example` |
| `IP_HASH_SALT` | Sal do hash de IP | Valor próprio exigido quando `production` | valor de desenvolvimento em `config.go` | sal fornecido fora do Git | Sim | `config.Load`, `.env.example` |
| `METRICS_TOKEN` | Protege `/metrics` se definido | Sim quando `production` | vazio | token fornecido fora do Git | Sim | `config.Load` |
| `TRUSTED_PROXIES` | IPs/CIDRs autorizados a fornecer `X-Forwarded-For` | Não | vazio | `192.0.2.10/32` | Não | `config.Load` |
| `CLICK_CONSUMER` | Nome do consumidor do Redis Stream | Não | `click-worker-<hostname>` | `click-worker-local` | Não | `config.Load` |
| `RATE_LIMIT_CREATE_PER_HOUR` | Criações por usuário por hora | Não | `100` | `100` | Não | `config.Load` |
| `RATE_LIMIT_AUTH_PER_MINUTE` | Tentativas por IP por minuto | Não | `20` | `20` | Não | `config.Load` |
| `RATE_LIMIT_REDIRECT_PER_MINUTE` | Redirects por IP por minuto | Não | `300` | `300` | Não | `config.Load` |
| `CACHE_TTL` | Vida da entrada de cache de link | Não | `24h` | `24h` | Não | `config.Load` |
| `MIGRATIONS_PATH` | Origem das migrações | Não | `file://migrations` | `file:///migrations` na imagem | Não | `config.Load`, `Dockerfile` |

`ENV=production` recusa `JWT_SECRET` e `IP_HASH_SALT` de desenvolvimento, exige token de métricas, JWT com pelo menos 32 caracteres e `PUBLIC_BASE_URL` HTTPS fora de localhost. Essas verificações de configuração **não** constituem um guia de implantação pública. Os três limites devem ser positivos; inteiros e durações inválidos em outras variáveis recaem no padrão observado (`internal/config/config.go`).

`TRUSTED_PROXIES` aceita uma lista separada por vírgulas de IPs ou CIDRs. O código só usa `X-Forwarded-For` quando o peer remoto está nessa lista. A [visão de segurança](../security/overview.md) explica o alcance dessa regra. Não coloque credenciais verdadeiras em `.env.example`, exemplos ou commits.
