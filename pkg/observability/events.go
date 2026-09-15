package observability

import (
	"context"
	"log/slog"
	"os"
)

// App é a instância global do agente New Relic, atribuída uma vez em
// cmd/api/main.go logo após a inicialização (App = NewRelicApp()). É
// deliberadamente global (em vez de injetada em cada usecase) pra não
// quebrar a assinatura de nenhum construtor já existente (e os testes
// unitários que os chamam diretamente, ex: usecase.NewOrderUseCase(...) nos
// arquivos *_test.go) — RecordOrderEvent trata App == nil como "New Relic
// desligado" e sempre grava o log estruturado independentemente disso, então
// nada quebra rodando sem a chave configurada (testes, CI, dev sem conta).
var App interface {
	RecordCustomEvent(eventType string, params map[string]interface{})
}

var eventLogger = slog.New(slog.NewJSONHandler(os.Stdout, nil))

// RecordOrderEvent registra um evento de ciclo de vida de OS — transição de
// status, aprovação/recusa, ou falha no processamento — tanto como evento
// customizado no New Relic (alimenta os dashboards de "volume diário de OS"
// e "tempo médio de execução por status" exigidos na Fase 3, via consultas
// NRQL sobre o evento "OrderLifecycle") quanto como log JSON estruturado
// (satisfaz "logs estruturados, incluindo correlação entre requisições" —
// correlationID deve vir do middleware.CorrelationID(c) sempre que chamado
// dentro de um handler HTTP).
func RecordOrderEvent(eventName, correlationID, orderID string, attrs map[string]interface{}) {
	params := map[string]interface{}{
		"correlationId": correlationID,
		"orderId":       orderID,
	}
	for k, v := range attrs {
		params[k] = v
	}

	if App != nil {
		App.RecordCustomEvent("OrderLifecycle", mergeEventName(eventName, params))
	}

	logAttrs := []any{"event", eventName, "correlation_id", correlationID, "order_id", orderID}
	for k, v := range attrs {
		logAttrs = append(logAttrs, k, v)
	}
	eventLogger.Log(context.Background(), slog.LevelInfo, "order_lifecycle", logAttrs...)
}

// RecordOrderFailure é a mesma coisa que RecordOrderEvent mas em nível ERROR
// — é o que alimenta o requisito específico de "alertas para falhas no
// processamento de ordens de serviço" (um alert condition no New Relic sobre
// o evento "OrderProcessingFailure" com count > 0 já cobre isso).
func RecordOrderFailure(step, correlationID, orderID, reason string) {
	params := map[string]interface{}{
		"correlationId": correlationID,
		"orderId":       orderID,
		"step":          step,
		"reason":        reason,
	}

	if App != nil {
		App.RecordCustomEvent("OrderProcessingFailure", params)
	}

	eventLogger.Log(context.Background(), slog.LevelError, "order_processing_failure",
		"correlation_id", correlationID, "order_id", orderID, "step", step, "reason", reason)
}

func mergeEventName(eventName string, params map[string]interface{}) map[string]interface{} {
	params["eventName"] = eventName
	return params
}
