# language: pt
Funcionalidade: Fluxo da ordem de serviço orquestrado pela Saga
  Para que a oficina atenda o cliente de ponta a ponta
  O OS Service conduz a ordem de serviço pelo Execution e pelo Billing
  E desfaz o que já foi feito quando algum passo falha

  Contexto:
    Dado que a peça "pastilha" tem 5 unidades em estoque

  Cenário: OS percorre todos os serviços até a finalização
    Dado uma OS aberta com o serviço "Troca de pastilhas" e 2 unidades da peça "pastilha"
    Então a OS deve estar "em_diagnostico"
    E o Execution Service deve ter recebido "StartDiagnosis"
    Quando o mecânico registra o diagnóstico "Pastilhas gastas"
    Então o Billing Service deve ter recebido "GenerateBudget"
    E a OS deve estar "aguardando_aprovacao"
    Quando o cliente aprova o orçamento
    Então a peça "pastilha" deve ter 3 unidades em estoque
    E a OS deve estar "aguardando_pagamento" com o link de pagamento
    Quando o pagamento é aprovado
    Então a OS deve estar "aguardando_execucao"
    E o Execution Service deve ter recebido "StartExecution"
    Quando o mecânico inicia o reparo
    Então a OS deve estar "em_execucao"
    Quando o mecânico finaliza o reparo
    Então a OS deve estar "finalizada"
    E a Saga deve estar "concluida"

  Cenário: cliente recusa o orçamento
    Dado uma OS aberta com o serviço "Troca de pastilhas" e 2 unidades da peça "pastilha"
    E o mecânico registra o diagnóstico "Pastilhas gastas"
    Quando o cliente recusa o orçamento
    Então a OS deve estar "recusada"
    E o orçamento deve estar cancelado
    E a peça "pastilha" deve ter 5 unidades em estoque
    E a Saga deve estar "cancelada"

  Cenário: falta de estoque cancela a OS e o orçamento
    Dado uma OS aberta com o serviço "Troca de pastilhas" e 10 unidades da peça "pastilha"
    E o mecânico registra o diagnóstico "Pastilhas gastas"
    Quando o cliente aprova o orçamento
    Então a OS deve estar "cancelada"
    E o orçamento deve estar cancelado
    E a peça "pastilha" deve ter 5 unidades em estoque
    E a Saga deve estar "cancelada"

  Cenário: pagamento recusado devolve as peças e cancela o orçamento
    Dado uma OS aberta com o serviço "Troca de pastilhas" e 2 unidades da peça "pastilha"
    E o mecânico registra o diagnóstico "Pastilhas gastas"
    E o cliente aprova o orçamento
    Quando o pagamento é recusado
    Então a OS deve estar "cancelada"
    E o Execution Service deve ter recebido "ReleaseParts"
    E a peça "pastilha" deve ter 5 unidades em estoque
    E o orçamento deve estar cancelado
    E a Saga deve estar "cancelada"

  Cenário: Execution não responde à reserva dentro do prazo
    Dado que o Execution Service não responde à reserva de peças
    E uma OS aberta com o serviço "Troca de pastilhas" e 2 unidades da peça "pastilha"
    E o mecânico registra o diagnóstico "Pastilhas gastas"
    E o cliente aprova o orçamento
    Quando o prazo do passo se esgota
    Então a OS deve estar "cancelada"
    E o Execution Service deve ter recebido "ReleaseParts"
    E o orçamento deve estar cancelado
    E a Saga deve estar "cancelada"
