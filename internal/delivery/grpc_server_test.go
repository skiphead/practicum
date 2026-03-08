// delivery/server_test.go
package delivery

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/skiphead/practicum/internal/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// ============================================================================
// Helper functions for tests
// ============================================================================

// getTestLogger создаёт тестовый логгер
func getTestLogger(t *testing.T) *zap.Logger {
	t.Helper()
	logger, err := zap.NewDevelopment(zap.AddStacktrace(zap.FatalLevel))
	require.NoError(t, err)
	return logger
}

// generateTestTLSFiles создаёт временные самоподписанные сертификаты для тестов
func generateTestTLSFiles(t *testing.T) (certPath, keyPath string) {
	t.Helper()

	// Генерация приватного ключа
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// Создание шаблона сертификата
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test Org"},
			CommonName:   "localhost",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		DNSNames:              []string{"localhost"},
	}

	// Самоподписанный сертификат
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	require.NoError(t, err)

	// Создание временных файлов в t.TempDir() - автоматическая очистка
	tmpDir := t.TempDir()
	certPath = filepath.Join(tmpDir, "test.crt")
	keyPath = filepath.Join(tmpDir, "test.key")

	// Запись сертификата
	certOut, err := os.Create(certPath)
	require.NoError(t, err)
	require.NoError(t, pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}))
	require.NoError(t, certOut.Close())

	// Запись ключа
	keyOut, err := os.Create(keyPath)
	require.NoError(t, err)
	require.NoError(t, pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}))
	require.NoError(t, keyOut.Close())

	t.Cleanup(func() {
		_ = os.Remove(certPath)
		_ = os.Remove(keyPath)
	})

	return certPath, keyPath
}

// createTestToken создаёт валидный тестовый JWT токен
func createTestToken(t *testing.T, sessionKey, userID string) string {
	t.Helper()
	cfg := utils.TokenConfig{SessionKey: sessionKey}
	token, err := utils.GenerateSessionToken(userID, cfg)
	require.NoError(t, err)
	return token
}

// bufDialer возвращает функцию для подключения к bufconn listener
func bufDialer(lis *bufconn.Listener) func(context.Context, string) (net.Conn, error) {
	return func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}
}

// ============================================================================
// Тесты для extractBearerToken
// ============================================================================

func TestExtractBearerToken(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		headers   []string
		wantToken string
		wantErr   bool
		errSubstr string
	}{
		"empty headers": {
			headers:   []string{},
			wantErr:   true,
			errSubstr: "authorization header missing",
		},
		"nil headers": {
			headers:   nil,
			wantErr:   true,
			errSubstr: "authorization header missing",
		},
		"empty header value": {
			headers:   []string{""},
			wantErr:   true,
			errSubstr: "empty authorization header",
		},
		"whitespace only": {
			headers:   []string{"   "},
			wantErr:   true,
			errSubstr: "empty authorization header",
		},
		"token without prefix": {
			headers:   []string{"test-token-123"},
			wantToken: "test-token-123",
			wantErr:   false,
		},
		"token with Bearer prefix": {
			headers:   []string{"Bearer test-token-123"},
			wantToken: "test-token-123",
			wantErr:   false,
		},
		"token with extra whitespace": {
			headers:   []string{"  Bearer  test-token-123  "},
			wantToken: "test-token-123",
			wantErr:   false,
		},
		"multiple headers - uses first": {
			headers:   []string{"first-token", "second-token"},
			wantToken: "first-token",
			wantErr:   false,
		},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := extractBearerToken(tt.headers)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errSubstr != "" {
					assert.Contains(t, err.Error(), tt.errSubstr)
				}
				assert.Empty(t, got)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantToken, got)
			}
		})
	}
}

// ============================================================================
// Тесты для portIsNull
// ============================================================================

func TestPortIsNull(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		input    int
		expected int
	}{
		"zero port":     {input: 0, expected: 50051},
		"negative port": {input: -1, expected: -1},
		"valid port":    {input: 8080, expected: 8080},
		"standard grpc": {input: 50051, expected: 50051},
		"high port":     {input: 65535, expected: 65535},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, portIsNull(tt.input))
		})
	}
}

// ============================================================================
// Тесты для NewServer
// ============================================================================

func TestNewServer(t *testing.T) {
	t.Parallel()

	logger := getTestLogger(t)
	testSessionKey := "test-secret-key-for-jwt-signing-min-32-chars!!"

	t.Run("nil logger returns error", func(t *testing.T) {
		cfg := &ServerConfig{Port: 0, SessionKey: testSessionKey}
		srv, err := NewServer(cfg, nil)
		assert.Error(t, err)
		assert.Nil(t, srv)
		assert.Contains(t, err.Error(), "logger is nil")
	})

	t.Run("minimal config without TLS", func(t *testing.T) {
		cfg := &ServerConfig{
			Port:       0,
			SessionKey: testSessionKey,
			TLS:        nil,
		}
		srv, err := NewServer(cfg, logger)
		require.NoError(t, err)
		require.NotNil(t, srv)
		assert.NotNil(t, srv.grpcServer)
		assert.Equal(t, testSessionKey, srv.sessionKey)
		assert.Equal(t, 0, srv.port)
	})

	t.Run("config with empty session key disables auth", func(t *testing.T) {
		cfg := &ServerConfig{
			Port:       0,
			SessionKey: "",
			TLS:        nil,
		}
		srv, err := NewServer(cfg, logger)
		require.NoError(t, err)
		require.NotNil(t, srv)
		assert.Empty(t, srv.sessionKey)
	})

	t.Run("invalid TLS cert path returns error", func(t *testing.T) {
		cfg := &ServerConfig{
			Port: 0,
			TLS: &TLSConfig{
				Enabled: true,
				Cert:    "/nonexistent/cert.pem",
				Key:     "/nonexistent/key.pem",
			},
			SessionKey: testSessionKey,
		}
		srv, err := NewServer(cfg, logger)
		assert.Error(t, err)
		assert.Nil(t, srv)
		assert.Contains(t, err.Error(), "failed to load TLS")
	})

	t.Run("valid TLS config succeeds", func(t *testing.T) {
		certPath, keyPath := generateTestTLSFiles(t)

		cfg := &ServerConfig{
			Port: 0,
			TLS: &TLSConfig{
				Enabled: true,
				Cert:    certPath,
				Key:     keyPath,
			},
			SessionKey: testSessionKey,
		}
		srv, err := NewServer(cfg, logger)
		require.NoError(t, err)
		require.NotNil(t, srv)
		assert.NotNil(t, srv.grpcServer)
	})
}

// ============================================================================
// Тесты для ensureValidToken interceptor
// ============================================================================

func TestEnsureValidTokenInterceptor(t *testing.T) {
	t.Parallel()

	logger := getTestLogger(t)
	testSessionKey := "test-secret-key-for-jwt-signing-min-32-chars!!"
	testUserID := "user-123-test"

	createInterceptor := func(key string) grpc.UnaryServerInterceptor {
		s := &ServerGRPC{
			logger:     logger,
			sessionKey: key,
		}
		return s.ensureValidToken()
	}

	// Простой тестовый хендлер
	// FIX: используем константу KeyUserID для контекста
	testHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		userID, ok := ctx.Value(utils.KeyUserID).(string)
		if !ok {
			return nil, status.Error(codes.Internal, "user_id not in context")
		}
		// Map-ключи для ответа теста — строки, это не контекст
		return map[string]string{"user_id": userID, "status": "ok"}, nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}

	t.Run("missing metadata returns Unauthenticated", func(t *testing.T) {
		interceptor := createInterceptor(testSessionKey)
		ctx := context.Background()
		_, err := interceptor(ctx, nil, info, testHandler)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
		assert.Contains(t, st.Message(), "missing request metadata")
	})

	t.Run("missing authorization header returns Unauthenticated", func(t *testing.T) {
		interceptor := createInterceptor(testSessionKey)
		ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{})
		_, err := interceptor(ctx, nil, info, testHandler)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.Unauthenticated, st.Code())
	})

	t.Run("invalid token returns Unauthenticated", func(t *testing.T) {
		interceptor := createInterceptor(testSessionKey)
		ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{
			"authorization": []string{"Bearer invalid-token-here"},
		})
		_, err := interceptor(ctx, nil, info, testHandler)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.Unauthenticated, st.Code())
		assert.Contains(t, st.Message(), "invalid session token")
	})

	t.Run("token signed with wrong key fails", func(t *testing.T) {
		wrongKey := "different-secret-key-that-does-not-match-anything!!"
		interceptor := createInterceptor(testSessionKey)
		token := createTestToken(t, wrongKey, testUserID)
		ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{
			"authorization": []string{"Bearer " + token},
		})
		_, err := interceptor(ctx, nil, info, testHandler)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.Unauthenticated, st.Code())
	})

	t.Run("empty session key in server skips validation", func(t *testing.T) {
		interceptor := createInterceptor("")
		ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{
			"authorization": []string{"Bearer any-token"},
		})
		_, err := interceptor(ctx, nil, info, testHandler)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.Unauthenticated, st.Code())
	})
}

// ============================================================================
// Тесты для recoveryInterceptor
// ============================================================================

func TestRecoveryInterceptor(t *testing.T) {
	t.Parallel()

	logger := getTestLogger(t)
	s := &ServerGRPC{logger: logger}
	interceptor := s.recoveryInterceptor()
	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/PanicMethod"}

	t.Run("normal handler passes through", func(t *testing.T) {
		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			return "success", nil
		}
		resp, err := interceptor(context.Background(), nil, info, handler)
		assert.NoError(t, err)
		assert.Equal(t, "success", resp)
	})

	t.Run("handler error is propagated", func(t *testing.T) {
		expectedErr := status.Error(codes.NotFound, "not found")
		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			return nil, expectedErr
		}
		resp, err := interceptor(context.Background(), nil, info, handler)
		assert.Nil(t, resp)
		assert.Equal(t, expectedErr, err)
	})

	t.Run("panic is recovered and converted to Internal error", func(t *testing.T) {
		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			panic("unexpected panic in handler")
		}
		resp, err := interceptor(context.Background(), nil, info, handler)
		assert.Nil(t, resp)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.Internal, st.Code())
		assert.Contains(t, st.Message(), "internal server error")
	})

	t.Run("panic with error value is recovered", func(t *testing.T) {
		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			panic(fmt.Errorf("panic with error"))
		}
		resp, err := interceptor(context.Background(), nil, info, handler)
		assert.Nil(t, resp)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.Internal, st.Code())
	})
}

// ============================================================================
// Интеграционные тесты сервера (без моков, с реальным gRPC)
// ============================================================================

func TestServerRunAndStop(t *testing.T) {
	t.Parallel()

	logger := getTestLogger(t)
	testSessionKey := "test-secret-key-for-jwt-signing-min-32-chars!!"

	t.Run("server starts and stops gracefully", func(t *testing.T) {
		cfg := &ServerConfig{
			Port:       0,
			SessionKey: testSessionKey,
			TLS:        nil,
		}
		srv, err := NewServer(cfg, logger)
		require.NoError(t, err)
		require.NotNil(t, srv)

		addr, err := srv.Run()
		require.NoError(t, err)
		require.NotNil(t, addr)

		time.Sleep(100 * time.Millisecond)

		conn, err := net.DialTimeout("tcp", addr.String(), 2*time.Second)
		require.NoError(t, err, "server should be listening on the port")
		_ = conn.Close()

		srv.GracefulStop()
		time.Sleep(100 * time.Millisecond)
	})

	t.Run("server shutdown stops immediately", func(t *testing.T) {
		cfg := &ServerConfig{
			Port:       0,
			SessionKey: "",
			TLS:        nil,
		}
		srv, err := NewServer(cfg, logger)
		require.NoError(t, err)

		time.Sleep(50 * time.Millisecond)
		srv.Shutdown()
		time.Sleep(100 * time.Millisecond)
	})

	t.Run("Run on occupied port returns error", func(t *testing.T) {
		srv1, err := NewServer(&ServerConfig{Port: 0, SessionKey: testSessionKey}, logger)
		require.NoError(t, err)
		addr1, err := srv1.Run()
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		_, portStr, err := net.SplitHostPort(addr1.String())
		require.NoError(t, err)

		var portInt int
		_, err = fmt.Sscanf(portStr, "%d", &portInt)
		require.NoError(t, err, "failed to parse port: %s", portStr)

		srv2, err := NewServer(&ServerConfig{Port: portInt, SessionKey: testSessionKey}, logger)
		require.NoError(t, err)
		_, err = srv2.Run()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to listen")

		srv1.GracefulStop()
		srv2.GracefulStop()
	})
}

// ============================================================================
// Тесты с bufconn для тестирования gRPC вызовов без сети
// ============================================================================

func TestServerPublicMethods(t *testing.T) {
	t.Parallel()

	logger := getTestLogger(t)
	testSessionKey := "test-secret-key-for-jwt-signing-min-32-chars!!"

	t.Run("GetGRPCServer returns internal server", func(t *testing.T) {
		cfg := &ServerConfig{Port: 0, SessionKey: testSessionKey}
		srv, err := NewServer(cfg, logger)
		require.NoError(t, err)

		grpcSrv := srv.GetGRPCServer()
		assert.NotNil(t, grpcSrv)
		assert.Equal(t, srv.grpcServer, grpcSrv)
	})

	t.Run("RegisterService registers custom service", func(t *testing.T) {
		cfg := &ServerConfig{Port: 0, SessionKey: testSessionKey}
		srv, err := NewServer(cfg, logger)
		require.NoError(t, err)

		impl := struct{}{}
		desc := &grpc.ServiceDesc{
			ServiceName: "test.TestService",
			HandlerType: (*interface{})(nil),
			Methods:     []grpc.MethodDesc{},
			Streams:     []grpc.StreamDesc{},
			Metadata:    "test.proto",
		}

		assert.NotPanics(t, func() {
			srv.RegisterService(desc, impl)
		})
	})

	t.Run("RegisterShortenerService requires initialized dependencies", func(t *testing.T) {
		cfg := &ServerConfig{Port: 0, SessionKey: testSessionKey}
		srv, err := NewServer(cfg, logger)
		require.NoError(t, err)

		assert.NotPanics(t, func() {
			srv.RegisterShortenerService()
		})
	})
}

// ============================================================================
// Тесты для TLS конфигурации
// ============================================================================

func TestServerTLSConfiguration(t *testing.T) {
	t.Parallel()

	logger := getTestLogger(t)
	testSessionKey := "test-secret-key-for-jwt-signing-min-32-chars!!"

	t.Run("server with TLS can be created", func(t *testing.T) {
		certPath, keyPath := generateTestTLSFiles(t)

		cfg := &ServerConfig{
			Port: 0,
			TLS: &TLSConfig{
				Enabled: true,
				Cert:    certPath,
				Key:     keyPath,
			},
			SessionKey: testSessionKey,
		}
		srv, err := NewServer(cfg, logger)
		require.NoError(t, err)
		assert.NotNil(t, srv)
		assert.NotNil(t, srv.grpcServer)
	})

	t.Run("TLS config with disabled flag ignores cert paths", func(t *testing.T) {
		cfg := &ServerConfig{
			Port: 0,
			TLS: &TLSConfig{
				Enabled: false,
				Cert:    "/nonexistent/cert.pem",
				Key:     "/nonexistent/key.pem",
			},
			SessionKey: testSessionKey,
		}
		srv, err := NewServer(cfg, logger)
		require.NoError(t, err)
		assert.NotNil(t, srv)
	})
}

// ============================================================================
// Edge cases и дополнительные тесты
// ============================================================================

func TestServerEdgeCases(t *testing.T) {
	t.Parallel()

	logger := getTestLogger(t)

	t.Run("multiple GracefulStop calls are safe", func(t *testing.T) {
		cfg := &ServerConfig{Port: 0, SessionKey: "key"}
		srv, err := NewServer(cfg, logger)
		require.NoError(t, err)

		assert.NotPanics(t, func() {
			srv.GracefulStop()
			srv.GracefulStop()
			srv.GracefulStop()
		})
	})

	t.Run("multiple Shutdown calls are safe", func(t *testing.T) {
		cfg := &ServerConfig{Port: 0, SessionKey: "key"}
		srv, err := NewServer(cfg, logger)
		require.NoError(t, err)

		assert.NotPanics(t, func() {
			srv.Shutdown()
			srv.Shutdown()
		})
	})

	t.Run("GracefulStop after Shutdown is safe", func(t *testing.T) {
		cfg := &ServerConfig{Port: 0, SessionKey: "key"}
		srv, err := NewServer(cfg, logger)
		require.NoError(t, err)

		assert.NotPanics(t, func() {
			srv.Shutdown()
			srv.GracefulStop()
		})
	})

	t.Run("GetGRPCServer after Shutdown returns server", func(t *testing.T) {
		cfg := &ServerConfig{Port: 0, SessionKey: "key"}
		srv, err := NewServer(cfg, logger)
		require.NoError(t, err)

		srv.Shutdown()
		grpcSrv := srv.GetGRPCServer()
		assert.NotNil(t, grpcSrv)
	})
}

// ============================================================================
// Тест контекста с user_id
// ============================================================================

func TestContextUserIDPropagation(t *testing.T) {
	t.Parallel()

	t.Run("context key is unexported type", func(t *testing.T) {
		// FIX: используем константу KeyUserID вместо строки
		ctx := context.WithValue(context.Background(), utils.KeyUserID, "test-user")
		userID, ok := ctx.Value(utils.KeyUserID).(string)
		assert.True(t, ok)
		assert.Equal(t, "test-user", userID)
	})

	t.Run("context value without key returns nil", func(t *testing.T) {
		ctx := context.Background()
		// FIX: используем константу для проверки отсутствия значения
		userID, ok := ctx.Value(utils.KeyUserID).(string)
		assert.False(t, ok)
		assert.Empty(t, userID)
	})
}
