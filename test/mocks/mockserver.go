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
			select {
			case err := <-m.ErrChan:
				if err != nil {
					m.StartFunc = func() error { return err }
				}
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
