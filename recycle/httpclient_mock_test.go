// mock_auditclient.go
package audit

import (
	"context"
	"errors"
	"testing"
)

// MockAuditClient для тестирования зависимостей
type MockAuditClient struct {
	CreateAuditEventFunc          func(ctx context.Context, req *CreateAuditRequest) error
	CreateAuditEventWithRetryFunc func(ctx context.Context, req *CreateAuditRequest, retryOpts ...RetryOption) error
	BatchCreateAuditEventsFunc    func(ctx context.Context, events []*CreateAuditRequest) error
}

func (m *MockAuditClient) CreateAuditEvent(ctx context.Context, req *CreateAuditRequest) error {
	if m.CreateAuditEventFunc != nil {
		return m.CreateAuditEventFunc(ctx, req)
	}
	return errors.New("not implemented")
}

func (m *MockAuditClient) CreateAuditEventWithRetry(ctx context.Context, req *CreateAuditRequest, retryOpts ...RetryOption) error {
	if m.CreateAuditEventWithRetryFunc != nil {
		return m.CreateAuditEventWithRetryFunc(ctx, req, retryOpts...)
	}
	return errors.New("not implemented")
}

func (m *MockAuditClient) BatchCreateAuditEvents(ctx context.Context, events []*CreateAuditRequest) error {
	if m.BatchCreateAuditEventsFunc != nil {
		return m.BatchCreateAuditEventsFunc(ctx, events)
	}
	return errors.New("not implemented")
}

// TestUsingMock демонстрирует использование мока
func TestUsingMock(t *testing.T) {
	mock := &MockAuditClient{
		CreateAuditEventFunc: func(ctx context.Context, req *CreateAuditRequest) error {
			if req.UserId == "testuser" {
				return nil
			}
			return errors.New("user not found")
		},
	}

	// Используем мок как интерфейс
	var client AuditClient = mock

	req := &CreateAuditRequest{
		Ts:     1234567890,
		Action: "login",
		UserId: "testuser",
		Url:    "http://example.com/login",
	}

	err := client.CreateAuditEvent(context.Background(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	req.UserId = "wronguser"
	err = client.CreateAuditEvent(context.Background(), req)
	if err == nil {
		t.Error("expected error, got nil")
	}
}
