# ADR-003: Hash de IP nos cliques

- **Status:** Aceito
- **Contexto:** O analytics precisa estimar visitantes únicos sem guardar o IP em texto puro na tabela de cliques.
- **Decisão:** Armazenar SHA-256 de salt e IP, com salt definido por ambiente. O hash permite contar valores distintos, mas não representa uma pessoa única nem garante anonimização completa.
- **Consequências:** Não é possível fazer geolocalização precisa a partir do valor armazenado. O servidor usa `X-Forwarded-For` apenas quando o peer está em `TRUSTED_PROXIES`; ignora `CF-Connecting-IP`.
