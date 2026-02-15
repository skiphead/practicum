// Package mocks provides mock implementations for testing components of the URL shortening service.
// It includes mock servers, clients, and other dependencies to isolate unit tests
// from external systems and simplify test setup and verification.
package mocks

import (
	"context"
)

type MockServer struct {
	StartFunc    func() error
	ShutdownFunc func(ctx context.Context) error
	ErrChan      chan error
}

func (m *MockServer) Start() error {
	if m.StartFunc != nil {
		return m.StartFunc()
	}

	if m.ErrChan != nil {
		go func() {
			err := <-m.ErrChan
			if err != nil {
				m.StartFunc = func() error { return err }
			}
		}()
	}
	return nil
}

func (m *MockServer) Shutdown(ctx context.Context) error {
	if m.ShutdownFunc != nil {
		return m.ShutdownFunc(ctx)
	}
	return nil
}
