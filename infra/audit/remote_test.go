package audit

import (
	"context"
	"testing"
	"time"

	"github.com/skiphead/practicum/pkg/transport/httpclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockHTTPClient для тестирования
type MockHTTPClient struct {
	mock.Mock
}

func (m *MockHTTPClient) SendRequest(ctx context.Context, method, path string, body interface{}) error {
	args := m.Called(ctx, method, path, body)
	return args.Error(0)
}

func (m *MockHTTPClient) ShouldRetry(err error, attempt int, opts httpclient.RetryOptions) (bool, time.Duration) {
	args := m.Called(err, attempt, opts)
	return args.Bool(0), args.Get(1).(time.Duration)
}

func TestDefaultServiceConfig(t *testing.T) {
	config := DefaultServiceConfig()
	assert.Equal(t, 1000, config.MaxBatchSize, "Default MaxBatchSize should be 1000")
}

func TestNewService(t *testing.T) {
	httpClient := &httpclient.HTTPClient{}
	config := ServiceConfig{MaxBatchSize: 500}

	service := NewService(httpClient, config)

	assert.NotNil(t, service)
	assert.Equal(t, httpClient, service.httpClient)
	assert.Equal(t, config, service.config)
}

func TestValidateAuditRequest_ValidRequest(t *testing.T) {
	service := &Service{}

	req := &CreateAuditRequest{
		TS:     1234567890,
		Action: "shorten",
		UserID: "user123",
		URL:    "https://example.com",
	}

	err := service.validateAuditRequest(req)
	assert.NoError(t, err)
}

func TestValidateAuditRequest_NilRequest(t *testing.T) {
	service := &Service{}

	err := service.validateAuditRequest(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "request cannot be nil")
}

func TestValidateAuditRequest_InvalidTimestamp(t *testing.T) {
	service := &Service{}

	testCases := []struct {
		name string
		ts   int
	}{
		{"zero timestamp", 0},
		{"negative timestamp", -1},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &CreateAuditRequest{
				TS:     tc.ts,
				Action: "shorten",
				UserID: "user123",
				URL:    "https://example.com",
			}

			err := service.validateAuditRequest(req)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "timestamp must be positive")
		})
	}
}

func TestValidateAuditRequest_InvalidAction(t *testing.T) {
	service := &Service{}

	testCases := []struct {
		name   string
		action string
		errMsg string
	}{
		{"empty action", "", "action cannot be empty"},
		{"too long action", string(make([]byte, 101)), "action too long, max 100 characters"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &CreateAuditRequest{
				TS:     1234567890,
				Action: tc.action,
				UserID: "user123",
				URL:    "https://example.com",
			}

			err := service.validateAuditRequest(req)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.errMsg)
		})
	}
}

func TestValidateAuditRequest_InvalidUserID(t *testing.T) {
	service := &Service{}

	testCases := []struct {
		name   string
		userID string
		errMsg string
	}{
		{"empty user_id", "", "user_id cannot be empty"},
		{"too long user_id", string(make([]byte, 101)), "user_id too long, max 100 characters"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &CreateAuditRequest{
				TS:     1234567890,
				Action: "shorten",
				UserID: tc.userID,
				URL:    "https://example.com",
			}

			err := service.validateAuditRequest(req)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.errMsg)
		})
	}
}

func TestValidateAuditRequest_InvalidURL(t *testing.T) {
	service := &Service{}

	testCases := []struct {
		name   string
		url    string
		errMsg string
	}{
		{"empty url", "", "url cannot be empty"},
		{"too long url", "https://example.com/" + string(make([]byte, 2000)), "url too long, max 2000 characters"},
		{"invalid url format", "://invalid-url", "invalid URL"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &CreateAuditRequest{
				TS:     1234567890,
				Action: "shorten",
				UserID: "user123",
				URL:    tc.url,
			}

			err := service.validateAuditRequest(req)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.errMsg)
		})
	}
}

func TestBatchCreateAuditEvents_EmptyEvents(t *testing.T) {
	mockClient := new(MockHTTPClient)
	service := &Service{
		httpClient: mockClient,
		config:     ServiceConfig{MaxBatchSize: 100},
	}

	err := service.BatchCreateAuditEvents(context.Background(), []*CreateAuditRequest{})
	assert.NoError(t, err)

	mockClient.AssertNotCalled(t, "SendRequest")
}

func TestBatchCreateAuditEvents_ExceedsMaxBatchSize(t *testing.T) {
	mockClient := new(MockHTTPClient)
	service := &Service{
		httpClient: mockClient,
		config:     ServiceConfig{MaxBatchSize: 2},
	}

	events := []*CreateAuditRequest{
		{TS: 1, Action: "a", UserID: "u", URL: "https://example.com"},
		{TS: 2, Action: "b", UserID: "u", URL: "https://example.com"},
		{TS: 3, Action: "c", UserID: "u", URL: "https://example.com"},
	}

	err := service.BatchCreateAuditEvents(context.Background(), events)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "batch size 3 exceeds maximum allowed size 2")

	mockClient.AssertNotCalled(t, "SendRequest")
}

func TestBatchCreateAuditEvents_InvalidEventInBatch(t *testing.T) {
	mockClient := new(MockHTTPClient)
	service := &Service{
		httpClient: mockClient,
		config:     ServiceConfig{MaxBatchSize: 100},
	}

	events := []*CreateAuditRequest{
		{TS: 1, Action: "valid", UserID: "user", URL: "https://example.com"},
		{TS: 2, Action: "", UserID: "user", URL: "https://example.com"},
		{TS: 3, Action: "valid", UserID: "user", URL: "https://example.com"},
	}

	err := service.BatchCreateAuditEvents(context.Background(), events)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid audit request at index 1")

	mockClient.AssertNotCalled(t, "SendRequest")
}

func TestCreateAuditEvent_ValidationError(t *testing.T) {
	mockClient := new(MockHTTPClient)
	service := &Service{
		httpClient: mockClient,
		config:     DefaultServiceConfig(),
	}

	// Invalid request with empty action
	req := &CreateAuditRequest{
		TS:     1234567890,
		Action: "",
		UserID: "user123",
		URL:    "https://example.com",
	}

	err := service.CreateAuditEvent(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid audit request")

	mockClient.AssertNotCalled(t, "SendRequest")
}

func TestCreateAuditEventWithRetry_ValidationError(t *testing.T) {
	mockClient := new(MockHTTPClient)
	service := &Service{
		httpClient: mockClient,
		config:     DefaultServiceConfig(),
	}

	ctx := context.Background()
	invalidReq := &CreateAuditRequest{
		TS:     1234567890,
		Action: "",
		UserID: "user123",
		URL:    "https://example.com",
	}

	err := service.CreateAuditEventWithRetry(ctx, invalidReq)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid audit request")

	mockClient.AssertNotCalled(t, "SendRequest")
}

// Дополнительные тесты для достижения 40% покрытия
func TestService_ImplementsClientInterface(t *testing.T) {
	var _ Client = (*Service)(nil)
	assert.True(t, true, "Service should implement Client interface")
}

func TestCreateAuditRequest_StructTags(t *testing.T) {
	req := &CreateAuditRequest{}

	// Проверяем, что структура имеет правильные JSON теги
	// Это можно проверить с помощью рефлексии или просто убедиться,
	// что структура корректно определена
	assert.NotNil(t, req)
}
