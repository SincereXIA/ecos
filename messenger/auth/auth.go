package auth

import (
	"context"
	"ecos/messenger/config"
	"ecos/messenger/tags"
	"encoding/base64"
	"errors"
	grpc_auth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"strings"
)

const RootUserID = "root"

//BasicAuthFunc is a middleware that extracts the Basic Auth from the request
func BasicAuthFunc(ctx context.Context) (context.Context, error) {
	token, err := grpc_auth.AuthFromMD(ctx, "basic")
	if err != nil {
		return nil, err
	}
	userID, err := parseToken(token)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "invalid auth token: %v", err)
	}
	grpc_ctxtags.Extract(ctx).Set(tags.TagUserID, userID)
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

func NewCtxWithToken(ctx context.Context, ak string, sk string) context.Context {
	md := metadata.Pairs("authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(ak+":"+sk)))
	nCtx := metautils.NiceMD(md).ToOutgoing(ctx)
	return nCtx
}

// GetUserID extracts the user_id from the context
func GetUserID(ctx context.Context) (string, error) {
	ts := grpc_ctxtags.Extract(ctx)
	conf := config.GetGrpcConfig(ctx)
	if conf.AuthEnabled {
		if ts.Has(tags.TagUserID) {
			return ts.Values()[tags.TagUserID].(string), nil
		} else {
			return "", errors.New("user_id not found")
		}
	}
	return RootUserID, nil
}
