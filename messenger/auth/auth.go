package auth

import (
	"context"
	"encoding/base64"
	grpc_auth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"strings"
)

func BasicAuthFunc(ctx context.Context) (context.Context, error) {
	token, err := grpc_auth.AuthFromMD(ctx, "basic")
	if err != nil {
		return nil, err
	}
	userID, err := parseToken(token)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "invalid auth token: %v", err)
	}
	grpc_ctxtags.Extract(ctx).Set("userID", userID)
	return ctx, nil
}

func parseToken(token string) (userID string, err error) {
	// Get user_id from base64 encoded token
	raw, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return "", err
	}
	splits := strings.SplitN(string(raw), ":", 2)
	if len(splits) != 2 {
		return "", err
	}
	return splits[0], nil
}
