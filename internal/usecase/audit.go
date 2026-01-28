package usecase

import (
	"encoding/json"
	"github.com/skiphead/practicum/internal/domain"
	"log"
	"sync"
)

// AuditManager - менеджер аудита (Издатель)
type AuditManager struct {
	subscribers map[string]domain.Subscriber
	mu          sync.RWMutex
}

// NewAuditManager - конструктор менеджера аудита
func NewAuditManager() *AuditManager {
	return &AuditManager{
		subscribers: make(map[string]domain.Subscriber),
	}
}

// Subscribe - подписаться на события
func (am *AuditManager) Subscribe(subscriber domain.Subscriber) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.subscribers[subscriber.ID()] = subscriber
	log.Printf("Подписчик %s добавлен", subscriber.ID())
}

// Unsubscribe - отписаться от событий
func (am *AuditManager) Unsubscribe(subscriberID string) {
	am.mu.Lock()
	defer am.mu.Unlock()
	delete(am.subscribers, subscriberID)
	log.Printf("Подписчик %s удален", subscriberID)
}

// Notify - уведомить всех подписчиков
func (am *AuditManager) Notify(event domain.AuditEvent) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	for _, subscriber := range am.subscribers {
		go subscriber.Update(event) // Асинхронная обработка
	}
}

// Logger - подписчик для логирования
type Logger struct {
	id string
}

func NewLogger(id string) *Logger {
	return &Logger{id: id}
}

func (l *Logger) Update(event domain.AuditEvent) {
	actionName := "создание ссылки"
	if event.Action == "follow" {
		actionName = "переход по ссылке"
	}

	log.Printf("[Logger %s] %s: пользователь=%s, url=%s",
		l.id, actionName, event.UserID, event.URL)
}

func (l *Logger) ID() string {
	return l.id
}

// MetricsCollector - подписчик для сбора метрик
type MetricsCollector struct {
	id            string
	totalShortens int64
	totalFollows  int64
	urlStats      map[string]int64
	userStats     map[string]int64
	mu            sync.RWMutex
}

func NewMetricsCollector(id string) *MetricsCollector {
	return &MetricsCollector{
		id:        id,
		urlStats:  make(map[string]int64),
		userStats: make(map[string]int64),
	}
}

func (mc *MetricsCollector) Update(event domain.AuditEvent) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if event.Action == "shorten" {
		mc.totalShortens++
	} else if event.Action == "follow" {
		mc.totalFollows++
	}

	mc.urlStats[event.URL]++

	if event.UserID != "" {
		mc.userStats[event.UserID]++
	}

	log.Printf("[MetricsCollector %s] Статистика: всего shorten=%d, follow=%d",
		mc.id, mc.totalShortens, mc.totalFollows)
}

func (mc *MetricsCollector) GetMetrics() map[string]interface{} {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	return map[string]interface{}{
		"total_shortens": mc.totalShortens,
		"total_follows":  mc.totalFollows,
		"total_events":   mc.totalShortens + mc.totalFollows,
		"unique_urls":    len(mc.urlStats),
		"unique_users":   len(mc.userStats),
	}
}

func (mc *MetricsCollector) ID() string {
	return mc.id
}

// AnalyticsExporter - подписчик для экспорта аналитики
type AnalyticsExporter struct {
	id       string
	endpoint string
}

func NewAnalyticsExporter(id, endpoint string) *AnalyticsExporter {
	return &AnalyticsExporter{
		id:       id,
		endpoint: endpoint,
	}
}

func (ae *AnalyticsExporter) Update(event domain.AuditEvent) {
	// Здесь можно добавить логику отправки во внешние системы
	// Например: Google Analytics, Yandex.Metrika, собственная БД

	data, _ := json.Marshal(event)
	log.Printf("[AnalyticsExporter %s] Экспорт в %s: %s",
		ae.id, ae.endpoint, string(data))
}

func (ae *AnalyticsExporter) ID() string {
	return ae.id
}

// ArchiveStorage - подписчик для архивации событий
type ArchiveStorage struct {
	id       string
	filePath string
}

func NewArchiveStorage(id, filePath string) *ArchiveStorage {
	return &ArchiveStorage{
		id:       id,
		filePath: filePath,
	}
}

func (as *ArchiveStorage) Update(event domain.AuditEvent) {
	// Здесь будет запись в файл или внешнее хранилище
	// Для примера просто логируем

	log.Printf("[ArchiveStorage %s] Архивация события в %s: action=%s, user=%s",
		as.id, as.filePath, event.Action, event.UserID)
}

func (as *ArchiveStorage) ID() string {
	return as.id
}
