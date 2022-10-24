package alaya

import (
	"context"
	"ecos/messenger/tags"
	"ecos/utils/errno"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"google.golang.org/grpc/metadata"
	"strconv"
)

func SetTermToContext(ctx context.Context, term uint64) (context.Context, error) {
	md := metadata.Pairs(tags.TagTerm, strconv.FormatUint(term, 10))
	newCtx := metadata.NewOutgoingContext(ctx, md)
	return newCtx, nil
}

func GetTermFromContext(ctx context.Context) (uint64, error) {
	val := metautils.ExtractIncoming(ctx).Get(tags.TagTerm)
	term, err := strconv.ParseUint(val, 10, 64)
	if err != nil {
		return 0, errno.TermNotExist
	}
	ts := grpc_ctxtags.Extract(ctx)
	ts.Set(tags.TagTerm, term)
	return term, nil
}

// SetTermToIncomingContext sets the term to the incoming context
// **Note**: This function is only used for testing
func SetTermToIncomingContext(ctx context.Context, term uint64) (context.Context, error) {
	md := metadata.Pairs(tags.TagTerm, strconv.FormatUint(term, 10))
	newCtx := metadata.NewIncomingContext(ctx, md)
	return newCtx, nil
}
