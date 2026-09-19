# ADR-004: Registro de clique assíncrono

- **Status:** Superado (v2)
- **Contexto:** Gravar o clique de forma síncrona no caminho do redirect adiciona latência e acopla a disponibilidade do redirect à do banco de analytics.
- **Decisão (MVP):** Canal Go em memória.
- **Decisão (v2):** Redis Streams (`XADD` no redirect, consumer group `click-workers`). Redirect continua sem esperar o `INSERT` no Postgres. Falha no `XADD` incrementa `clicks_dropped` e ainda devolve 302.
- **Consequências:** Eventos sobrevivem a crash do processo da API enquanto o Redis persistir; há atraso eventual até o ACK.
