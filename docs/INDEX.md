# Índice da documentação

## Conhecer e usar

1. [Apresentação do Encurta](README.md): objetivo e possibilidades, sem configuração técnica.
2. [Capacidades e limites](product/capabilities.md): regras e estágio atual.
3. [Guia de uso e solução de problemas](user-guide/using.md): principais tarefas no painel local.

## Desenvolver e manter

1. [README técnico](../README.md): entrada rápida para o repositório.
2. [Desenvolvimento local](development/local.md): ambiente, comandos e testes.
3. [Arquitetura e dados](architecture/overview.md): responsabilidades e fluxo do clique.
4. [API HTTP](reference/api.md) e [OpenAPI](../internal/web/openapi.yaml): autenticação, exemplos e contrato.
5. [Configuração](reference/configuration.md): variáveis e padrões observados.
6. [Segurança](security/overview.md): proteções implementadas e limites.
7. [ADRs](adr/001-linguagem-go.md): decisões registradas; leia também [Base62](adr/002-codigo-base62.md), [IP](adr/003-ip-hash.md), [cliques](adr/004-cliques-assincronos.md), [cache](adr/005-cache-aside.md) e [JWT](adr/006-jwt.md).

## Operar o ambiente local

1. [Operação local](operations/local.md): início, saúde, métricas, paradas e falhas conhecidas.
2. [Estado e lacunas da documentação](maintenance/documentation-status.md): evidências, verificações e próximos passos.

Os arquivos em `docs/handoffs/` e `docs/reviews/` são registros internos de trabalho. Não integram este caminho público de leitura nem comprovam, sozinhos, o estado de um ambiente em execução.
