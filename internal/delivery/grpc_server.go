package delivery

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strings"

	"github.com/skiphead/practicum/internal/audit"
	"github.com/skiphead/practicum/internal/delivery/handler"
	"github.com/skiphead/practicum/internal/pkg/utils"
	"github.com/skiphead/practicum/internal/usecase"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	pb "github.com/skiphead/practicum/pkg/api/v1/gen"
)

// TLSConfig содержит настройки TLS для gRPC сервера.
type TLSConfig struct {
	Enabled bool
	Cert    string
	Key     string
}

// ServerConfig содержит полную конфигурацию gRPC сервера.
type ServerConfig struct {
	Port       int
	SessionKey string // ← переименовано из Token для согласованности с utilits
	TLS        *TLSConfig
}

// ServerGRPC представляет собой универсальный gRPC сервер.
type ServerGRPC struct {
	grpcServer  *grpc.Server
	auditClient *audit.Adapter
	port        int
	baseURL     string
	useCase     usecase.URLUseCase
	logger      *zap.Logger
	sessionKey  string // ← добавлено поле для ключа сессии
}

// extractBearerToken извлекает токен из заголовка Authorization.
// Поддерживает форматы: "Bearer <token>" и просто "<token>".
// Возвращает очищенный токен или ошибку.
func extractBearerToken(authHeaders []string) (string, error) {
	if len(authHeaders) < 1 {
		return "", fmt.Errorf("authorization header missing")
	}

	tokenStr := strings.TrimSpace(authHeaders[0])
	if tokenStr == "" {
		return "", fmt.Errorf("empty authorization header")
	}

	// Удаляем префикс "Bearer " если есть
	const bearerPrefix = "Bearer "
	if strings.HasPrefix(strings.ToLower(tokenStr), "bearer ") {
		tokenStr = strings.TrimPrefix(tokenStr, bearerPrefix)
	}

	tokenStr = strings.TrimSpace(tokenStr)
	if tokenStr == "" {
		return "", fmt.Errorf("empty token after prefix removal")
	}

	return tokenStr, nil
}

// ensureValidToken возвращает интерцептор gRPC для валидации JWT-токена.
func (s *ServerGRPC) ensureValidToken() grpc.UnaryServerInterceptor {
	// Методы, не требующие аутентификации
	publicMethods := map[string]bool{
		"/auth.AuthService/CreateToken": true,
	}

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		logger := s.logger.With(
			zap.String("operation", "grpc_jwt_auth"),
			zap.String("method", info.FullMethod),
		)

		// Проверяем, требует ли метод аутентификации
		if publicMethods[info.FullMethod] {
			logger.Debug("Public method - skipping authentication")
			return handler(ctx, req)
		}

		// Извлекаем метаданные
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			logger.Warn("Metadata not found in request")
			return nil, status.Errorf(codes.InvalidArgument, "missing request metadata")
		}

		// Извлекаем токен из заголовка
		tokenString, err := extractBearerToken(md["authorization"])
		if err != nil {
			logger.Warn("Failed to extract token", zap.Error(err))
			return nil, status.Errorf(codes.Unauthenticated, "invalid or missing authorization header")
		}

		// Валидируем JWT-токен через utilits
		cfg := utils.TokenConfig{SessionKey: s.sessionKey}
		claims, err := utils.ParseSessionToken(tokenString, cfg)
		if err != nil {
			logger.Warn("JWT validation failed",
				zap.Error(err),
				zap.String("token_preview", tokenString[:min(10, len(tokenString))]+"..."),
			)
			return nil, status.Errorf(codes.Unauthenticated, "invalid session token: %v", err)
		}

		if existing, ok := ctx.Value(utils.KeyUserID).(string); ok && existing != "" {
			logger.Debug("UserID already present in context", zap.String("user_id", existing))
		}

		ctxWithUser := context.WithValue(ctx, utils.KeyUserID, claims.UserID)

		logger.Debug("JWT authentication successful",
			zap.String("user_id", claims.UserID),
		)

		// Выполняем обработчик с обновлённым контекстом
		return handler(ctxWithUser, req)

	}
}

// recoveryInterceptor — интерцептор для перехвата паник.
func (s *ServerGRPC) recoveryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				s.logger.Error("Panic recovered in gRPC handler",
					zap.Any("panic", r),
					zap.String("method", info.FullMethod),
					zap.Stack("stack"),
				)
				err = status.Errorf(codes.Internal, "internal server error")
			}
		}()
		return handler(ctx, req)
	}
}

// portIsNull возвращает порт по умолчанию, если передан 0.
func portIsNull(port int) int {
	if port == 0 {
		return 50051
	}
	return port
}

// NewServer создаёт и настраивает новый экземпляр gRPC сервера.
func NewServer(cfg *ServerConfig, logger *zap.Logger) (*ServerGRPC, error) {
	if logger == nil {
		return nil, fmt.Errorf("new server initial error - logger is nil")
	}

	logger = logger.With(
		zap.String("service", "grpc_server"),
		zap.String("component", "delivery"),
	)

	var opts []grpc.ServerOption

	// Настройка TLS
	if cfg.TLS != nil && cfg.TLS.Enabled {
		logger.Info("Loading TLS configuration",
			zap.String("cert_file", cfg.TLS.Cert),
			zap.String("key_file", cfg.TLS.Key),
		)

		certificate, err := tls.LoadX509KeyPair(cfg.TLS.Cert, cfg.TLS.Key)
		if err != nil {
			logger.Error("Failed to load TLS certificate", zap.Error(err))
			return nil, fmt.Errorf("failed to load TLS cert and key: %w", err)
		}
		opts = append(opts, grpc.Creds(credentials.NewServerTLSFromCert(&certificate)))
	}

	// Создаём экземпляр сервера
	s := &ServerGRPC{
		port:       cfg.Port,
		logger:     logger,
		sessionKey: cfg.SessionKey, // ← сохраняем ключ сессии
	}

	// Цепочка интерцепторов
	var interceptors []grpc.UnaryServerInterceptor
	interceptors = append(interceptors, s.recoveryInterceptor())

	// Добавляем JWT-интерцептор, если задан sessionKey
	if cfg.SessionKey != "" {
		interceptors = append(interceptors, s.ensureValidToken())
		logger.Info("JWT authentication interceptor enabled")
	} else {
		logger.Warn("Authentication disabled - session key not configured")
	}

	if len(interceptors) > 0 {
		opts = append(opts, grpc.ChainUnaryInterceptor(interceptors...))
	}

	grpcServer := grpc.NewServer(opts...)
	reflection.Register(grpcServer)
	s.grpcServer = grpcServer

	logger.Info("gRPC server instance created",
		zap.Int("port", cfg.Port),
		zap.Bool("tls_enabled", cfg.TLS != nil && cfg.TLS.Enabled),
		zap.Bool("auth_enabled", cfg.SessionKey != ""),
	)
	return s, nil
}

// RegisterShortenerService регистрирует сервис подсетей.
func (s *ServerGRPC) RegisterShortenerService() {
	shortServer := handler.NewShortenerHandler(s.useCase, s.baseURL, s.auditClient, s.logger)
	pb.RegisterShortenerServiceServer(s.grpcServer, shortServer)
	s.logger.Info("Subnet service registered",
		zap.String("service_name", "ShortenerService"),
	)
}

// RegisterAuthService регистрирует сервис подсетей.
func (s *ServerGRPC) RegisterAuthService() {
	authServer := handler.NewAuthHandler(s.auditClient, s.sessionKey, s.logger)
	pb.RegisterAuthServiceServer(s.grpcServer, authServer)
	s.logger.Info("Subnet service registered",
		zap.String("service_name", "ShortenerService"),
	)
}

// RegisterService регистрирует произвольный gRPC сервис.
func (s *ServerGRPC) RegisterService(desc *grpc.ServiceDesc, impl interface{}) {
	s.grpcServer.RegisterService(desc, impl)
	s.logger.Info("Custom gRPC service registered",
		zap.String("service_name", desc.ServiceName),
	)
}

// Run запускает gRPC сервер.
func (s *ServerGRPC) Run() (net.Addr, error) {
	port := portIsNull(s.port)
	address := fmt.Sprintf(":%d", port)

	lis, err := net.Listen("tcp", address)
	if err != nil {
		s.logger.Error("Failed to listen",
			zap.String("address", address),
			zap.Error(err))
		return nil, fmt.Errorf("failed to listen on port %d: %w", port, err)
	}

	s.logger.Info("Starting gRPC server",
		zap.String("address", lis.Addr().String()),
		zap.Int("port", port),
	)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				s.logger.Error("PANIC in gRPC server runtime",
					zap.Any("panic", r),
					zap.Stack("stack"),
					zap.String("stage", "server_runtime"),
				)
			}
		}()

		s.logger.Info("gRPC server accepting connections")
		if err := s.grpcServer.Serve(lis); err != nil {
			s.logger.Error("gRPC server runtime error",
				zap.String("stage", "runtime"),
				zap.Error(err))
		}
	}()

	return lis.Addr(), nil
}

// GracefulStop плавно останавливает сервер.
func (s *ServerGRPC) GracefulStop() {
	if s.grpcServer != nil {
		s.logger.Info("Initiating graceful shutdown of gRPC server")
		s.grpcServer.GracefulStop()
		s.logger.Info("gRPC server stopped gracefully")
	}
}

// Shutdown немедленно останавливает сервер.
func (s *ServerGRPC) Shutdown() {
	if s.grpcServer != nil {
		s.logger.Warn("Initiating immediate shutdown of gRPC server")
		s.grpcServer.Stop()
		s.logger.Info("gRPC server stopped immediately")
	}
}

// GetGRPCServer возвращает внутренний gRPC сервер.
func (s *ServerGRPC) GetGRPCServer() *grpc.Server {
	return s.grpcServer
}
