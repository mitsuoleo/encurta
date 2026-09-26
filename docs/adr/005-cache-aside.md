# ADR-005: Cache-aside em vez de write-through

- **Status:** Aceito
- **Contexto:** Precisamos decidir como o Redis é populado.
- **Decisão:** Cache-aside — a criação grava apenas no PostgreSQL. Em um miss no redirect, a leitura mantém `SELECT ... FOR SHARE` até terminar o `SET` no Redis. Update/deactivate fazem a alteração na mesma transação que mantém o lock exclusivo da linha, apagam a entrada Redis antes do commit e revertem a transação se a invalidação falhar. A ordem de locks é PostgreSQL → Redis.
- **Consequências:** A primeira leitura após um cache miss (ex.: restart do Redis) é mais lenta. Uma escrita pode esperar a população do cache em outra instância. Falha no `SET` durante leitura deixa o PostgreSQL como fonte de verdade; falha no `DEL` impede a escrita. Nenhum caminho grava no cache após o commit da escrita, evitando repopulação com versão obsoleta.
