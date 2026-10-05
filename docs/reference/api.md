# API HTTP

A especificação [OpenAPI 3.0](../../internal/web/openapi.yaml), servida em `/openapi.yaml` quando a aplicação está em execução, é a referência canônica de caminhos, corpos e respostas. Esta página explica como usá-la e chama atenção para regras que afetam clientes. As rotas são registradas em `internal/handler/http.go`; os tipos de resposta de analytics estão em `internal/domain/link.go`.

## Autenticação e uso

`POST /auth/register` cria uma conta e retorna `201`; `POST /auth/login` retorna `200`. Ambos recebem JSON com `email` e `password` e devolvem `access_token`, `expires_in` (segundos) e `email`. A senha aceita de 8 a 72 bytes. Envie `Authorization: Bearer <access_token>` para criar, listar, editar, desativar ou consultar analytics de links. `POST /auth/logout` retorna `204`, mas não revoga o token no servidor.

Um exemplo local no PowerShell, com dados fictícios e Compose iniciado:

```powershell
'{"email":"exemplo@example.com","password":"senha-ficticia-123"}' | Set-Content -NoNewline auth.json
$resposta = curl.exe -sS -X POST http://localhost:8080/auth/register -H "Content-Type: application/json" --data-binary "@auth.json" | ConvertFrom-Json
$token = $resposta.access_token
'{"url":"https://example.com/pagina","alias":"meu-link"}' | Set-Content -NoNewline link.json
curl.exe -sS -X POST http://localhost:8080/links -H "Content-Type: application/json" -H "Authorization: Bearer $token" --data-binary "@link.json"
```

Os arquivos `auth.json` e `link.json` contêm valores de exemplo; remova-os após o uso e não substitua os exemplos por credenciais reais em arquivos versionados. O cadastro falhará se o e-mail ou alias já estiver em uso.

## Comportamentos relevantes

- `POST /links` recebe `url`, `alias` opcional e `expires_at` opcional em RFC3339. Retorna `short_code` e `short_url`. O destino e o alias seguem as [regras do produto](../product/capabilities.md).
- `GET /links` retorna `{links,total,limit,offset}` para o dono. Limite padrão: 50; máximo: 100. Limites não positivos voltam ao padrão e offsets negativos viram zero (`internal/service/link.go`).
- `PATCH /links/{code}` aceita `url`, `expires_at` e `is_active`. `expires_at: ""` remove a validade. `DELETE /links/{code}` desativa o link; a exclusão física não ocorre.
- `GET /links/{code}/analytics` retorna contagens do dono. Para link inexistente ou de outra conta, a API responde `404` sem revelar a propriedade.
- `GET /{code}` é público. Retorna `302` para um link ativo, `410` se inativo/vencido, `404` se inexistente e `429` ao exceder o limite de redirects. O registro do clique é assíncrono.
- Erros da API usam JSON com a chave `error`; `/health` retorna um objeto `status` com `app`, `db` e `cache` (`internal/handler/http.go`).

Uma falha do Redis usada apenas na verificação do limite de redirects deixa o pedido passar; falhas no limite de autenticação ou criação retornam `503` (`internal/middleware/ratelimit.go`). Consulte [segurança](../security/overview.md) e [operação local](../operations/local.md).
