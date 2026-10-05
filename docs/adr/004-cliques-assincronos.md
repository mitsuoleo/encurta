# ADR-004: Registro de clique assíncrono

- **Status:** Aceito (v2; substitui o canal em memória do MVP)
- **Contexto:** Gravar o clique de forma síncrona no caminho do redirect adiciona latência e acopla a disponibilidade do redirect à do banco de analytics.
- **Decisão (MVP):** Canal Go em memória.
- **Decisão (v2):** Redis Streams (`XADD` no redirect, consumer group `click-workers`, `MAXLEN` aproximado). Redirect continua sem esperar o `INSERT` no Postgres. Falha no `XADD` incrementa `clicks_dropped` e ainda devolve 302. Cada instância usa um consumer único (`click-worker-<hostname>`) e `XAUTOCLAIM` para mensagens pendentes após crash. Entrega é at-least-once (ACK depois do insert).
- **Consequências:** Eventos sobrevivem a crash do processo da API enquanto o Redis persistir; há atraso eventual até o ACK.
