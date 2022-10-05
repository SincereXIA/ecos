package rainbow

import (
	"ecos/messenger"
	"ecos/utils/logger"
)

type Rainbow struct {
	UnimplementedRainbowServer
}

func (r *Rainbow) GetStream(stream Rainbow_GetStreamServer) error {
	for {
		content, err := stream.Recv()
		if err != nil {
			return err
		}
		switch payload := content.Payload.(type) {
		case *Content_Request:
			logger.Infof("get request_seq: %v", payload.Request.RequestSeq)
			err := stream.Send(&Content{
				Payload: &Content_Response{
					Response: &Response{ResponseTo: payload.Request.RequestSeq},
				},
			})
			if err != nil {
				return err
			}
		case *Content_Response:
			logger.Infof("get response_to: %v", payload.Response.ResponseTo)
		}
	}
}

func NewRainbow(rpcServer *messenger.RpcServer) *Rainbow {
	rainbow := &Rainbow{}
	RegisterRainbowServer(rpcServer, rainbow)
	return rainbow
}
