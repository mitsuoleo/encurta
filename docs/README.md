# Encurta: links curtos para compartilhar

O Encurta transforma um endereço longo em um link curto. Você pode escolher um nome para o link, definir até quando ele funciona e acompanhar seus acessos em um painel. O projeto é um **portfólio de software**: a configuração disponível neste repositório roda no computador de quem o instala; não há um serviço público oferecido aqui.

![Painel atual do Encurta com dados ilustrativos](ui-dashboard.png)

*Imagem da interface atual com dados fictícios fornecidos por uma API simulada.*

Depois que o ambiente local estiver iniciado, abra `http://localhost:8080`, crie uma conta e informe o endereço de destino. O painel mostra o link para copiar e um código QR. Em **Seus links**, você pode abrir **Detalhes** para alterar o destino ou a validade, desativar ou reativar um link e consultar os acessos. O link curto redireciona quem o abre sem exigir conta; a gestão e os dados de acesso ficam disponíveis apenas ao dono da conta.

O painel apresenta total de cliques, estimativa de visitantes únicos, evolução por dia, origens de acesso, tipos de dispositivo e navegadores. O registro dos cliques ocorre em segundo plano, por isso um acesso recente pode demorar a aparecer. Um link desativado ou vencido deixa de redirecionar. Sair do painel encerra a sessão no navegador, mas um token já emitido continua válido até expirar.

Para aprender a usar o painel, siga o [guia de uso](user-guide/using.md). Se você vai instalar, desenvolver ou avaliar o projeto, comece pelo [README técnico](../README.md). O [índice completo](INDEX.md) reúne as rotas de leitura.

**Limites atuais:** não há implantação pública suportada, verificação de reputação dos destinos nem garantia de que todo clique seja registrado quando a fila de eventos falha. Consulte [capacidades e limites](product/capabilities.md) para detalhes confirmados no código.
