# Usar o painel local

Este guia pressupõe que o ambiente foi iniciado conforme o [README do repositório](../../README.md). Abra `http://localhost:8080` no navegador.

## Entrar e criar um link

1. Em **Criar conta**, informe e-mail e senha de pelo menos oito caracteres. Depois, use **Entrar** nas visitas seguintes.
2. Em **Encurte uma URL**, cole um destino iniciado por `http://` ou `https://`, por exemplo `https://example.com/pagina`.
3. Se quiser, escolha um alias como `meu-link` e uma validade futura. Deixe-os vazios para receber um código automático sem prazo.
4. Selecione **Criar link**. Use **Copiar link** ou o código QR mostrado no resultado para compartilhar.

O alias não pode estar em uso. Ele aceita de 3 a 20 letras, números ou hífens, sem hífen nas pontas nem dois hífens seguidos. Um destino local ou privado escrito como IP é recusado. Consulte [capacidades e limites](../product/capabilities.md) para as demais regras.

## Gerenciar e acompanhar

Em **Seus links**, selecione **Detalhes** para ver o destino, a validade e os acessos. **Editar destino e validade** permite atualizar a URL, definir outra data futura ou limpar a data para remover a validade. **Desativar link** pede confirmação; um link desativado deixa de redirecionar. Para reativá-lo, use **Reativar link**. Se ele também estiver vencido, ajuste ou remova a validade primeiro.

Os dados de acesso mostram total, visitantes únicos estimados, cliques por dia, referências, dispositivos e navegadores. Use **Atualizar dados** para consultar novamente. O registro ocorre em segundo plano: não espere que um clique apareça instantaneamente. **Carregar mais** busca a próxima página da lista.

O botão no topo alterna o tema claro e escuro. **Sair** remove a sessão do navegador. Um token já emitido ainda pode ser aceito pela API até expirar; não compartilhe o token.

## Quando algo não funciona

| Sintoma | Causa ou verificação útil | Próximo passo |
| --- | --- | --- |
| Não consigo entrar | E-mail ou senha incorretos, ou sessão expirada | Confira os dados e entre novamente; cadastro e login também têm limite de tentativas |
| O destino é recusado | Formato inválido, esquema diferente de HTTP(S) ou endereço local/privado | Use um endereço público completo e confira a [regra de URLs](../product/capabilities.md) |
| O alias não é aceito | Formato inválido, nome reservado ou já usado | Ajuste o nome; se estiver em uso, escolha outro |
| O link responde `410` | Ele foi desativado ou venceu | Abra **Detalhes**, ajuste a validade se necessário e reative |
| O clique ainda não aparece | Analytics é gravado em segundo plano | Aguarde e selecione **Atualizar dados**; se persistir, consulte a [operação local](../operations/local.md) |
| Lista ou analytics falharam | A API ou uma dependência local pode estar indisponível | Use **Tentar novamente** ou **Atualizar dados**; quem opera o ambiente pode conferir `/health` |

Para detalhes de status HTTP e autenticação, consulte a [referência da API](../reference/api.md).
