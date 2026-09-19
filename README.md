# URL Shortener com Analytics

Encurtador de URLs em Go: API REST + UI simples, JWT, cache Redis, PostgreSQL, rate limiting e cliques via Redis Streams.

## O que está pronto

- RF01–RF09: criar (API e UI), redirect 302, alias, expiração, cliques, analytics do dono, JWT, 410 para inativo/expirado, soft delete
- RNF04: 100 criações/hora por usuário (fallback IP)
- RNF07: Docker Compose + env
- Redirect: p95 ~33ms e ~3200 RPS em teste local (ver abaixo)

Fora de escopo: Grafana (métricas já em `/metrics`).

## Subir o ambiente

```bash
make up
```

UI e API: `http://localhost:8080`. Postgres e Redis só na rede do Compose.

```bash
make down
```

## Variáveis de ambiente

| Variável | Padrão | Descrição |
|----------|--------|-----------|
| `HTTP_ADDR` | `:8080` | Bind HTTP |
| `DATABASE_URL` | `postgres://shortener:shortener@localhost:5432/shortener?sslmode=disable` | Postgres |
| `REDIS_URL` | `redis://localhost:6379/0` | Redis |
| `PUBLIC_BASE_URL` | `http://localhost:8080` | Base de `short_url` |
| `IP_HASH_SALT` | `dev-only-change-me` | Salt SHA-256 do IP |
| `JWT_SECRET` | `dev-jwt-change-me` | Assinatura HS256 |
| `MIGRATIONS_PATH` | `file://migrations` | (`file:///migrations` no container) |
| `RATE_LIMIT_CREATE_PER_HOUR` | `100` | Limite de `POST /links` |
| `CACHE_TTL` | `24h` | TTL código→URL no Redis |

## API

No PowerShell, use `--data-binary` com um arquivo JSON (o `-d` inline quebra).

### Health

```bash
curl -s http://localhost:8080/health
```

### Registro e login

```bash
curl -s -X POST http://localhost:8080/auth/register \
  -H 'Content-Type: application/json' \
  --data-binary '{"email":"voce@example.com","password":"password1"}'

curl -s -X POST http://localhost:8080/auth/login \
  -H 'Content-Type: application/json' \
  --data-binary '{"email":"voce@example.com","password":"password1"}'
```

Resposta: `{ "access_token", "expires_in", "email" }`. Use `Authorization: Bearer <token>` nas rotas autenticadas.

### Criar link

```bash
curl -s -X POST http://localhost:8080/links \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $TOKEN" \
  --data-binary '{"url":"https://example.com/caminho","alias":"mylink","expires_at":"2027-01-01T00:00:00Z"}'
```

`alias` e `expires_at` são opcionais. Alias inválido ou expiração no passado → `400`. Alias ocupado → `409`.

### Listar / desativar

```bash
curl -s http://localhost:8080/links -H "Authorization: Bearer $TOKEN"
curl -s -X DELETE http://localhost:8080/links/mylink -H "Authorization: Bearer $TOKEN"
```

Delete é soft delete (`is_active=false`). Redirect seguinte → `410 Gone`.

### Redirect (público)

```bash
curl -sS -D - -o /dev/null http://localhost:8080/mylink
```

`302` + `Location`. Clique vai para o stream Redis; o redirect não espera o Postgres.

### Analytics (dono)

```bash
curl -s http://localhost:8080/links/mylink/analytics -H "Authorization: Bearer $TOKEN"
```

Outro usuário → `403`.

### UI

Abra `http://localhost:8080`: criar conta, encurtar, copiar, ver barras de cliques por dia, desativar.

### Métricas

```bash
curl -s http://localhost:8080/metrics
```

## Testes e lint

```bash
make test
make lint
```

## Carga no redirect

Medido em 2026-09-19 (API no Docker, Windows + Docker Desktop), cache aquecido, **sem seguir o 302**.

| Ferramenta | Carga | RPS | Latência |
|------------|-------|-----|----------|
| hey `-z 10s -c 50 -disable-redirects` | 10s | **3215** | p95 **32.8ms** |
| wrk `-t4 -c50 -d10s --latency` | 10s | **3152** | p90 29.2ms, p99 60.5ms |

Criar o código agora exige JWT; o GET de redirect continua público.

```bash
docker run --rm williamyeh/hey -z 10s -c 50 -disable-redirects \
  "http://host.docker.internal:8080/${CODE}"
```

## Limitações

- Grafana não está no compose; `/metrics` já é Prometheus.
- JWT não é revogável antes do expiry.
- Rate limit falha aberto se o Redis cair na checagem.
- Alias só aceita `[0-9A-Za-z]`, 3–20 caracteres (sem hífen).

## ADRs

Ver [docs/adr](docs/adr).
