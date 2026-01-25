package audit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/skiphead/practicum/infra/audit"
	"github.com/skiphead/practicum/pkg/transport/httpclient"
)

// Config конфигурация адаптера
type Config struct {
	FilePath     string // Путь к файлу для записи аудита
	HTTPEndpoint string // URL для отправки аудита по HTTP
	Enabled      bool   // Включен ли аудит
	MaxBatchSize int    // Максимальный размер батча для HTTP
	QueueSize    int    // Размер очереди событий
}

// DefaultConfig возвращает конфигурацию по умолчанию
func DefaultConfig() Config {
	return Config{
		FilePath:     "/var/log/audit.log",
		HTTPEndpoint: "",
		Enabled:      true,
		MaxBatchSize: 1000,
		QueueSize:    10000,
	}
}

// Validate проверяет корректность конфигурации
func (c Config) Validate() error {
	// Убираем проверку на наличие хотя бы одного приемника
	// Если оба пустые, адаптер будет работать в "no-op" режиме

	if c.MaxBatchSize <= 0 {
		return fmt.Errorf("MaxBatchSize must be positive")
	}

	if c.QueueSize <= 0 {
		return fmt.Errorf("QueueSize must be positive")
	}

	return nil
}

// Adapter адаптирует события для отправки в разные приемники
type Adapter struct {
	fileLogger *audit.Logger
	httpClient audit.AuditClient
	config     Config
	enabled    bool
	mutex      sync.RWMutex
	queue      chan *Event
	wg         sync.WaitGroup
	done       chan struct{}
}

// NewAdapter создает новый адаптер аудита
func NewAdapter(cfg Config) (*Adapter, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	adapter := &Adapter{
		config:  cfg,
		enabled: cfg.Enabled,
		queue:   make(chan *Event, cfg.QueueSize),
		done:    make(chan struct{}),
	}

	var err error

	// Инициализируем файловый логгер только если указан путь
	if cfg.FilePath != "" {
		adapter.fileLogger, err = audit.GetInstance(cfg.FilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to create file logger: %w", err)
		}
	}

	// Инициализируем HTTP клиент только если указан endpoint
	if cfg.HTTPEndpoint != "" {
		httpCfg := httpclient.DefaultConfig()
		httpCfg.BaseURL = cfg.HTTPEndpoint
		httpCfg.MaxRetries = 3
		httpCfg.Timeout = 5

		httpClient, err := httpclient.New(httpCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create HTTP client: %w", err)
		}

		adapter.httpClient = audit.NewService(httpClient, audit.ServiceConfig{
			MaxBatchSize: cfg.MaxBatchSize,
		})
	}

	// Запускаем обработчик очереди, если аудит включен
	// и есть хотя бы один приемник
	if cfg.Enabled && (cfg.FilePath != "" || cfg.HTTPEndpoint != "") {
		adapter.startProcessor()
	} else if cfg.Enabled {
		// Если включен, но приемники не настроены, логируем это
		// но не создаем очередь и процессор
		adapter.enabled = false
	}

	return adapter, nil
}

// LogEvent добавляет событие в очередь для обработки
func (a *Adapter) LogEvent(ctx context.Context, event *Event) error {
	a.mutex.RLock()
	if !a.enabled {
		a.mutex.RUnlock()
		return nil
	}
	a.mutex.RUnlock()

	select {
	case a.queue <- event:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Очередь переполнена, пропускаем событие
		return fmt.Errorf("audit queue is full, event dropped")
	}
}

// startProcessor запускает обработчик очереди событий
func (a *Adapter) startProcessor() {
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()

		// Если нет HTTP клиента, не создаем батчи
		if a.httpClient == nil {
			a.processWithoutBatching()
			return
		}

		// Обработка с батчингом для HTTP
		a.processWithBatching()
	}()
}

// processWithoutBatching обрабатывает события без батчинга
func (a *Adapter) processWithoutBatching() {
	for {
		select {
		case <-a.done:
			return

		case event := <-a.queue:
			// Отправляем только в файл
			if err := a.logToFile(event); err != nil {
				// Логируем ошибку, но продолжаем
			}
		}
	}
}

// processWithBatching обрабатывает события с батчингом для HTTP
func (a *Adapter) processWithBatching() {
	batch := make([]*audit.CreateAuditRequest, 0, a.config.MaxBatchSize)
	batchTimer := time.NewTimer(1 * time.Second)

	defer batchTimer.Stop()

	for {
		select {
		case <-a.done:
			// Отправляем оставшиеся события перед завершением
			a.flushBatch(batch)
			return

		case event := <-a.queue:
			// Отправляем в файл (синхронно)
			if err := a.logToFile(event); err != nil {
				// Логируем ошибку, но продолжаем
			}

			// Добавляем в батч для HTTP
			if a.httpClient != nil {
				req := a.convertToAuditRequest(event)
				batch = append(batch, req)

				// Если батч достиг максимального размера, отправляем
				if len(batch) >= a.config.MaxBatchSize {
					a.flushBatchHTTP(batch)
					batch = batch[:0]
					batchTimer.Reset(1 * time.Second)
				}
			}

		case <-batchTimer.C:
			// Отправляем накопленные события по таймеру
			if len(batch) > 0 {
				a.flushBatchHTTP(batch)
				batch = batch[:0]
			}
			batchTimer.Reset(1 * time.Second)
		}
	}
}

// flushBatch отправляет батч событий
func (a *Adapter) flushBatch(batch []*audit.CreateAuditRequest) {
	if len(batch) == 0 {
		return
	}

	// Отправляем по HTTP
	a.flushBatchHTTP(batch)
}

// flushBatchHTTP отправляет батч событий по HTTP
func (a *Adapter) flushBatchHTTP(batch []*audit.CreateAuditRequest) {
	if a.httpClient == nil || len(batch) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := a.httpClient.BatchCreateAuditEvents(ctx, batch); err != nil {
		// Логируем ошибку, но не прерываем выполнение
		// В продакшене можно реализовать dead letter queue
	}
}

// logToFile записывает событие в файл
func (a *Adapter) logToFile(event *Event) error {
	if a.fileLogger == nil {
		return nil
	}

	switch event.Action {
	case "shorten":
		return a.fileLogger.LogShorten(event.UserID, event.URL)
	case "follow":
		return a.fileLogger.LogFollow(event.UserID, event.URL)
	default:
		return a.fileLogger.Log(audit.LogAction(event.Action), event.UserID, event.URL)
	}
}

// convertToAuditRequest конвертирует событие в формат аудит-модуля
func (a *Adapter) convertToAuditRequest(event *Event) *audit.CreateAuditRequest {
	return &audit.CreateAuditRequest{
		Ts:     int(event.Timestamp),
		Action: event.Action,
		UserId: event.UserID,
		Url:    event.URL,
	}
}

// Enable включает аудит
func (a *Adapter) Enable() {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if !a.enabled {
		a.enabled = true
		// Запускаем процессор только если есть приемники
		if a.config.FilePath != "" || a.config.HTTPEndpoint != "" {
			a.startProcessor()
		}
	}
}

// Disable выключает аудит
func (a *Adapter) Disable() {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if a.enabled {
		a.enabled = false
		close(a.done)
		a.wg.Wait()
		a.done = make(chan struct{})
	}
}

// Close закрывает адаптер
func (a *Adapter) Close() error {
	a.Disable()

	if a.fileLogger != nil {
		return a.fileLogger.Close()
	}

	return nil
}
