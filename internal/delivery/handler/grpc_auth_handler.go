package handler

import (
	"context"
	"fmt"

	"github.com/skiphead/practicum/internal/audit"
	"github.com/skiphead/practicum/internal/pkg/utils"
	pb "github.com/skiphead/practicum/pkg/api/v1/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type Auth struct {
	pb.UnimplementedAuthServiceServer
	auditClient audit.Logger
	sessionKey  string
	logger      *zap.Logger
}

func NewAuthHandler(auditClient *audit.Adapter, sessionKey string, logger *zap.Logger) pb.AuthServiceServer {
	return &Auth{
		auditClient: auditClient,
		sessionKey:  sessionKey,
		logger:      logger,
	}
}

func (h *Auth) CreateToken(_ context.Context, req *pb.CreateTokenRequest) (*pb.CreateTokenResponse, error) {
	if req.GetUserId() == "" {
		return &pb.CreateTokenResponse{}, status.Error(codes.InvalidArgument, "invalid user id")
	}

	cfg := TokenConfig{SessionKey: h.sessionKey}

	token, err := utils.GenerateSessionToken(req.GetUserId(), utils.TokenConfig(cfg))
	if err != nil {
		return &pb.CreateTokenResponse{}, status.Error(codes.InvalidArgument, err.Error())
	}

	bearerToken := fmt.Sprintf("Bearer %s", token)

	return &pb.CreateTokenResponse{
		BearerToken: proto.String(bearerToken),
	}, nil
}
