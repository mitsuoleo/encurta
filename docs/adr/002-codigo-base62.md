# ADR-002: Geração de código curto — Base62 aleatório vs. hash da URL

- **Status:** Aceito
- **Contexto:** Poderíamos gerar o código a partir de um hash da URL (determinístico) ou de forma aleatória.
- **Decisão:** Código aleatório base62 de 7 caracteres, com retry em caso de colisão (constraint UNIQUE no banco garante correção mesmo sob concorrência).
- **Alternativas consideradas:** Hash MD5/SHA truncado da URL — rejeitado porque duas pessoas encurtando a mesma URL gerariam o mesmo código, impedindo múltiplos "donos" do mesmo destino.
- **Consequências:** Necessário checar colisão antes de persistir (baixa probabilidade, mas deve ser tratada).
