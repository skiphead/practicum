package handler

import (
	"context"
	"fmt"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/skiphead/practicum/internal/audit"
	"github.com/skiphead/practicum/internal/domain/entity"
	"github.com/skiphead/practicum/internal/usecase"
	pb "github.com/skiphead/practicum/pkg/api/v1/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Shortener struct {
	pb.UnimplementedShortenerServiceServer
	storage     usecase.URLUseCase
	baseURL     string
	auditClient audit.Logger
	logger      *zap.Logger
}

func NewShortenerHandler(storage usecase.URLUseCase, baseURL string, auditClient *audit.Adapter, logger *zap.Logger) pb.ShortenerServiceServer {
	return &Shortener{
		storage:     storage,
		baseURL:     baseURL,
		auditClient: auditClient,
		logger:      logger,
	}
}

func (h *Shortener) ListUserURLs(ctx context.Context, req *pb.ListUserURLsRequest) (*pb.ListUserURLsResponse, error) {

	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	userID, ok := GetUserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "user id is required")
	}

	list, getErr := h.storage.GetByUserID(ctx, userID)
	if getErr != nil {
		h.logger.Error("list urls failed", zap.Error(getErr), zap.String("user_id", userID))
		return nil, status.Error(codes.Internal, "failed to retrieve URLs")
	}

	return &pb.ListUserURLsResponse{
		Url: h.convertListUrlsToProtoListURLs(list),
	}, nil
}

func (h *Shortener) CreateURLShorten(ctx context.Context, req *pb.CreateURLShortenRequest) (*pb.CreateURLShortenResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}
	if req.GetOriginalUrl() == "" {
		return nil, status.Error(codes.InvalidArgument, "original_url is required")
	}

	userID, ok := GetUserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "user id is required")
	}

	resp, err := h.storage.Save(ctx, req.GetOriginalUrl(), userID)
	if err != nil {
		if h.storage.IsDuplicateError(err) {
			return nil, status.Error(codes.AlreadyExists, "URL already shortened for this user")
		}
		h.logger.Error("save error", zap.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	h.logAuditEvent(ctx, audit.ActionShorten, userID, resp.OriginalURL)

	return &pb.CreateURLShortenResponse{
		ShortUrl: proto.String(fmt.Sprintf("%s/%s", h.baseURL, resp.ShortCode)),
	}, nil
}

func (h *Shortener) ExpandURL(ctx context.Context, req *pb.URLExpandRequest) (*pb.URLExpandResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}
	if req.GetShortCode() == "" {
		return nil, status.Error(codes.InvalidArgument, "short code is required")
	}

	data, getErr := h.storage.Get(ctx, req.GetShortCode())
	if getErr != nil {
		h.logger.Error("storage error", zap.Error(getErr))
		return nil, status.Error(codes.Internal, "failed to process request")
	}

	h.logAuditEvent(ctx, audit.ActionFollow, data.UserID, data.OriginalURL)

	return &pb.URLExpandResponse{
		OriginalUrl: proto.String(data.OriginalURL),
	}, nil
}

func (h *Shortener) convertListUrlsToProtoListURLs(list []entity.ShortURL) []*pb.URLData {
	if len(list) == 0 {
		return []*pb.URLData{}
	}

	urls := make([]*pb.URLData, 0, len(list))
	for _, url := range list {
		shortenedURL := fmt.Sprintf("%s/%s", h.baseURL, url.ShortCode)

		urls = append(urls, &pb.URLData{
			ShortUrl:    proto.String(shortenedURL),
			OriginalUrl: proto.String(url.OriginalURL),
		})
	}

	return urls
}

// Приватный метод
func (h *Shortener) logAuditEvent(ctx context.Context, action, userID, url string) {
	if h.auditClient == nil {
		return
	}
	auditCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	go func(ctx context.Context) {
		defer cancel()
		defer func() {
			if r := recover(); r != nil {
				h.logger.Error("audit panic", zap.Any("recover", r), zap.String("action", action))
			}
		}()
		if err := h.auditClient.LogEvent(ctx, &audit.Event{
			Timestamp: time.Now().Unix(),
			Action:    action,
			UserID:    userID,
			URL:       url,
		}); err != nil {
			h.logger.Warn("audit failed", zap.Error(err), zap.String("action", action))
		}
	}(auditCtx)
}
