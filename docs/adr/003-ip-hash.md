# ADR-003: Anonimização de IP nos cliques

- **Status:** Aceito
- **Contexto:** Guardar IP puro é passivo de problemas de privacidade/LGPD.
- **Decisão:** Armazenar apenas hash do IP (SHA-256 + salt fixo por ambiente), suficiente para detectar cliques duplicados sem expor dado pessoal identificável diretamente.
- **Consequências:** Não é possível fazer geolocalização precisa por IP sem um passo adicional de enriquecimento antes do hash (trade-off aceito para o MVP).
