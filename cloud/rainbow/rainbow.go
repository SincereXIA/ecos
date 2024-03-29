package rainbow

import (
	"context"
	"ecos/cloud/config"
	"ecos/edge-node/infos"
	"ecos/messenger"
	"ecos/messenger/common"
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
	router *Router // router 注册从本地发出的响应

	requestSeq uint64 // 主动从 cloud 发起的 request 序列号

	rwMutex sync.RWMutex // protect clusterInfo & requestSeq

	gaia  *CloudGaia
	alaya *CloudAlaya
	moon  *CloudMoon
}

// eventLoop 处理 stream 中收到的 content
func (r *Rainbow) eventLoop(stream Rainbow_GetStreamServer, nodeInfo *infos.NodeInfo, sendChan chan *Response) error {
	for {
		content, err := stream.Recv()
		if err != nil {
			logger.Warningf("rainbow to: %v stream recv error: %v and return", nodeInfo.RaftId, err)
			close(sendChan)
			return err
		}
		switch payload := content.Payload.(type) {
		case *Content_Response:
			r.router.Send(payload.Response.ResponseTo, payload.Response)
			if payload.Response.IsLast {
				r.router.Unregister(payload.Response.ResponseTo)
			}
		case *Content_Request:
			// TODO: rainbow 收到边缘节点请求，进行处理
			logger.Infof("get request_seq: %v", payload.Request.RequestSeq)
			if payload.Request.Method == Request_PUT && payload.Request.Resource == Request_INFO {
				if payload.Request.Info.GetInfoType() == infos.InfoType_CLUSTER_INFO {
					logger.Infof("get cluster info, term: %v", payload.Request.Info.GetClusterInfo().Term)
					if r.clusterInfo == nil || r.clusterInfo.Term <= payload.Request.Info.GetClusterInfo().Term {
						r.rwMutex.Lock()
						r.clusterInfo = payload.Request.Info.GetClusterInfo()
						r.rwMutex.Unlock()
					}
					continue
				}
			}
			switch payload.Request.Resource {
			case Request_META:
				r.processMetaRequest(payload.Request, sendChan)
			default:
				sendError(sendChan, payload.Request, errors.New("not support resource"))
			}
		}
	}
}

func (r *Rainbow) sendChanLoop(stream Rainbow_GetStreamServer, sendChan chan *Response) {
	for resp := range sendChan {
		logger.Debugf("cloud send response_to: %v", resp.ResponseTo)
		err := stream.Send(&Content{
			Payload: &Content_Response{
				Response: resp,
			},
		})
		if err != nil {
			logger.Errorf("send response error: %v", err)
			return
		}
	}
}

func sendError(sendChan chan *Response, req *Request, err error) {
	sendChan <- &Response{
		ResponseTo: req.RequestSeq,
		IsLast:     true,
		Result: &common.Result{
			Status:  common.Result_FAIL,
			Code:    -1,
			Message: err.Error(),
		},
	}
}

func (r *Rainbow) processMetaRequest(req *Request, sendChan chan *Response) {
	switch req.Method {
	case Request_DELETE:
		err := r.alaya.doDelete(req.RequestId)
		if err != nil {
			sendError(sendChan, req, err)
		}
		sendChan <- &Response{
			ResponseTo: req.RequestSeq,
			IsLast:     true,
		}
	}
	sendError(sendChan, req, errors.New("not support method"))
}

func (r *Rainbow) GetStream(stream Rainbow_GetStreamServer) error {
	for {
		content, err := stream.Recv() // 第一次交换，接受 nodeInfo
		if err != nil {
			return err
		}
		switch payload := content.Payload.(type) {
		case *Content_Request:
			payloadInfo := payload.Request.Info
			nodeInfo := payloadInfo.GetNodeInfo()
			logger.Infof("get stream connect, node: %v", nodeInfo.RaftId)

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
			sendChan := make(chan *Response, 10)
			go r.sendChanLoop(stream, sendChan)
			return r.eventLoop(stream, nodeInfo, sendChan)
		case *Content_Response:
			logger.Infof("get response_to: %v", payload.Response.ResponseTo)
		}
	}
}

// SendRequest 向边缘集群 leader 发送 request 请求
func (r *Rainbow) SendRequest(request *Request, stream Rainbow_SendRequestServer) error {
	respChan, err := r.SendRequestToEdgeLeader(request)
	if err != nil {
		return err
	}
	for resp := range respChan {
		if err := stream.Send(resp); err != nil {
			return err
		}
	}
	return nil
}

// SendRequestDirect 直接向边缘集群发送请求
func (r *Rainbow) SendRequestDirect(request *Request) (<-chan *Response, error) {
	respChan, err := r.SendRequestToEdgeLeader(request)
	if err != nil {
		return nil, err
	}
	return respChan, nil
}

func (r *Rainbow) GetClusterInfo() *infos.ClusterInfo {
	r.rwMutex.RLock()
	defer r.rwMutex.RUnlock()
	return r.clusterInfo
}

// SendRequestToNode 向指定节点发送请求
// 返回响应 channel
func (r *Rainbow) SendRequestToNode(nodeId uint64, request *Request) (<-chan *Response, error) {
	stream, ok := r.streams.Load(nodeId)
	if !ok {
		return nil, errors.New("stream not found")
	}

	r.rwMutex.Lock()
	request.RequestSeq = r.requestSeq
	r.requestSeq += 1
	r.rwMutex.Unlock()

	respChan := r.router.Register(request.RequestSeq)

	// 此处的操作需要同步
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

func NewRainbow(ctx context.Context, rpcServer *messenger.RpcServer, conf *config.CloudConfig) *Rainbow {
	rainbow := &Rainbow{
		router:     NewRouter(),
		requestSeq: 10000,
	}
	rainbow.gaia = NewCloudGaia(ctx, rpcServer, conf)
	rainbow.alaya = NewCloudAlaya(ctx, rpcServer, conf, rainbow)
	rainbow.moon = NewCloudMoon(ctx, rpcServer, rainbow)
	RegisterRainbowServer(rpcServer, rainbow)
	return rainbow
}
