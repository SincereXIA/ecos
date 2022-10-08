package rainbow

import (
	"ecos/edge-node/infos"
	"ecos/messenger"
	"ecos/utils/logger"
	"errors"
	"go.etcd.io/etcd/pkg/v3/wait"
	"sync"
)

type Rainbow struct {
	UnimplementedRainbowServer
	streams     sync.Map
	clusterInfo *infos.ClusterInfo

	w      wait.Wait
	router *Router

	requestSeq uint64
}

func (r *Rainbow) eventLoop(stream Rainbow_GetStreamServer, nodeInfo *infos.NodeInfo) error {
	for {
		content, err := stream.Recv()
		if err != nil {
			logger.Warningf("rainbow to: %v stream recv error: %v and return", nodeInfo.RaftId, err)
			return err
		}
		switch payload := content.Payload.(type) {
		case *Content_Response:
			r.router.Send(payload.Response.ResponseTo, payload.Response)
		case *Content_Request:
			// TODO: rainbow 收到边缘节点请求，进行处理
			logger.Infof("get request_seq: %v", payload.Request.RequestSeq)
			if payload.Request.Method == Request_PUT && payload.Request.Resource == Request_INFO {
				if payload.Request.Info.GetInfoType() == infos.InfoType_CLUSTER_INFO {
					logger.Infof("get cluster info, term: %v", payload.Request.Info.GetClusterInfo().Term)
					if r.clusterInfo == nil || r.clusterInfo.Term <= payload.Request.Info.GetClusterInfo().Term {
						r.clusterInfo = payload.Request.Info.GetClusterInfo()
					}
				}
			}
			// todo: reply
		}
	}
}

func (r *Rainbow) GetStream(stream Rainbow_GetStreamServer) error {
	for {
		content, err := stream.Recv() // 第一次交换，接受 nodeInfo
		if err != nil {
			return err
		}
		switch payload := content.Payload.(type) {
		case *Content_Request:
			logger.Infof("get request_seq: %v", payload.Request.RequestSeq)

			payloadInfo := payload.Request.Info
			nodeInfo := payloadInfo.GetNodeInfo()

			err := stream.Send(&Content{
				Payload: &Content_Response{
					Response: &Response{
						ResponseTo: payload.Request.RequestSeq,
						IsLast:     true,
					},
				},
			})
			if err != nil {
				return err
			}
			// stream 保存
			r.streams.Store(nodeInfo.RaftId, stream)

			return r.eventLoop(stream, nodeInfo)

		case *Content_Response:
			logger.Infof("get response_to: %v", payload.Response.ResponseTo)
		}
	}
}

func (r *Rainbow) SendRequest(request *Request, stream Rainbow_SendRequestServer) error {
	respChan, err := r.SendRequestToEdgeLeader(request)
	if err != nil {
		return err
	}
	for resp := range respChan {
		if err := stream.Send(resp); err != nil {
			return err
		}
		if resp.IsLast { // 最后一个响应，删除注册
			r.router.Unregister(resp.ResponseTo)
			break
		}
	}
	return nil
}

func (r *Rainbow) GetClusterInfo() *infos.ClusterInfo {
	return r.clusterInfo
}

func (r *Rainbow) SendRequestToNode(nodeId uint64, request *Request) (<-chan *Response, error) {
	stream, ok := r.streams.Load(nodeId)
	if !ok {
		return nil, errors.New("stream not found")
	}

	request.RequestSeq = r.requestSeq
	r.requestSeq += 1
	respChan := r.router.Register(request.RequestSeq)

	return respChan, stream.(Rainbow_GetStreamServer).Send(&Content{
		Payload: &Content_Request{
			Request: request,
		},
	})
}

func (r *Rainbow) SendRequestToEdgeLeader(request *Request) (<-chan *Response, error) {
	clusterInfo := r.GetClusterInfo()
	if clusterInfo.LeaderInfo == nil {
		return nil, errors.New("cluster leader not found")
	}
	leader := clusterInfo.LeaderInfo.RaftId

	return r.SendRequestToNode(leader, request)
}

func NewRainbow(rpcServer *messenger.RpcServer) *Rainbow {
	rainbow := &Rainbow{
		router:     NewRouter(),
		requestSeq: 10000,
	}
	RegisterRainbowServer(rpcServer, rainbow)
	return rainbow
}
