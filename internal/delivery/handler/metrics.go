package handler

import (
	"encoding/json"
	"github.com/skiphead/practicum/internal/usecase"
	"net/http"
)

// MetricsHandler - обработчик метрик
type MetricsHandler struct {
	metricsCollector *usecase.MetricsCollector
}

func NewMetricsHandler(metricsCollector *usecase.MetricsCollector) *MetricsHandler {
	return &MetricsHandler{metricsCollector: metricsCollector}
}

// HandleMetrics - получение метрик
func (mh *MetricsHandler) HandleMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := mh.metricsCollector.GetMetrics()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"metrics": metrics,
	})
}

// HandleStats - детальная статистика
func (mh *MetricsHandler) HandleStats(w http.ResponseWriter, r *http.Request) {
	// Здесь можно возвращать более детальную статистику
	// например, по временным интервалам, популярным URL и т.д.

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"stats": map[string]interface{}{
			"message": "Детальная статистика будет здесь",
		},
	})
}
