# Operar o Compose local

Este procedimento se aplica somente ao ambiente de desenvolvimento em `docker-compose.yml`. Não há um caminho de implantação pública vigente neste repositório. O processo Go aplica migrações automaticamente ao iniciar e precisa de PostgreSQL e Redis disponíveis (`cmd/api/main.go`).

## Iniciar e conferir

Pré-condições: Docker Desktop ou Docker Engine com Compose disponíveis e porta 8080 livre. Na raiz do repositório, em PowerShell:

```powershell
.\make.ps1 up
docker compose ps
curl.exe -sS http://localhost:8080/health
```

`up` constrói a imagem e inicia API, PostgreSQL e Redis. Um ambiente saudável deve responder `200` com `{"status":{"app":"ok","db":"ok","cache":"ok"}}`; a ordem das chaves pode variar. O código devolve `503` se o ping ao banco ou ao Redis falhar (`internal/handler/http.go`). Não foi feita execução desse procedimento nesta entrega.

Para acompanhar mensagens JSON da API, use `.\make.ps1 logs`; para parar sem apagar o volume do banco, use `.\make.ps1 down`. Não use `docker compose down -v` se os dados locais precisarem ser preservados. O Redis do Compose atual não tem volume declarado; não trate seus eventos e cache como um backup. As migrações sob `migrations/` são aplicadas durante a inicialização; a API encerra se falharem.

## Métricas locais

Opcionalmente, inicie o perfil de observabilidade:

```powershell
docker compose --profile obs up --build -d
docker compose --profile obs ps
```

Grafana fica em `http://localhost:3000`; Prometheus consulta `api:8080/metrics` a cada 15 segundos no perfil local (`deploy/prometheus.yml`). O painel provisionado está em `deploy/grafana/`. `/metrics` é público quando `METRICS_TOKEN` está vazio, como no Compose local; se o token for definido, exige `Authorization: Bearer <token>` (`internal/handler/http.go`). Métricas próprias incluem latência do redirect, cache hits/misses, erros de criação e cliques perdidos na publicação (`internal/observability/metrics.go`). O repositório não declara alertas, metas de disponibilidade ou política de retenção operacional.

As métricas próprias agora usam o prefixo `encurta_`, e o painel local foi atualizado para consultá-las. Consultas externas que ainda usem `urlshortener_` precisam ser ajustadas; dados históricos com nomes antigos não são renomeados automaticamente.

## Diagnosticar falhas

| Sinal | Verificação | Recuperação local conhecida |
| --- | --- | --- |
| API não inicia | `docker compose ps` e `.\make.ps1 logs`; procure erros de configuração, migração, PostgreSQL ou Redis | Corrija a configuração ou disponibilidade da dependência e reinicie a API com `docker compose up --build -d` |
| `/health` responde `503` | Confira `status.db` e `status.cache` e os logs do serviço correspondente | Restabeleça a dependência local; confira novamente `/health` |
| Redirect funciona mas clique não aparece | Consulte logs do worker e a métrica `encurta_clicks_dropped_total` | Aguarde o processamento; se a publicação falhou, aquele clique pode não ser recuperável |
| UI ou API responde `429` | Limite Redis para o IP ou usuário foi excedido | Aguarde a janela do limite; veja os valores em [configuração](../reference/configuration.md) |
| Porta 8080 ocupada | Verifique o processo que usa a porta e a publicação no Compose | Libere a porta ou ajuste a configuração local de forma coordenada |

Não há procedimento de backup, restauração ou rollback validado para este projeto. `down` apenas para os contêineres; não é uma estratégia de recuperação. Antes de usar dados que importem, será necessário definir e testar esses procedimentos separadamente. Consulte [lacunas](../maintenance/documentation-status.md).
