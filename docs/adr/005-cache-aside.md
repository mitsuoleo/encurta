# ADR-005: Cache-aside em vez de write-through

- **Status:** Aceito
- **Contexto:** Precisamos decidir como o Redis é populado.
- **Decisão:** Cache-aside — na criação do link já populamos o cache; em caso de miss no redirect, busca no Postgres e popula o cache (lazy).
- **Consequências:** Primeira leitura após um cache-miss (ex.: restart do Redis) é mais lenta; aceitável dado o volume esperado.
