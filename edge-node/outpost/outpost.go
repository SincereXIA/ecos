package outpost

import (
	"context"
	"ecos/client"
	"ecos/client/config"
	"ecos/cloud/rainbow"
	"ecos/edge-node/infos"
	"ecos/edge-node/watcher"
	"ecos/messenger"
	"ecos/messenger/common"
	"ecos/utils/logger"
	"errors"
	"math/rand"
	"strconv"
	"strings"
)

type Outpost struct {
	ctx context.Context

	cloudAddr  string
	requestSeq uint64

	w *watcher.Watcher
	r *rainbow.Router

	sendChan chan *rainbow.Content
	stream   rainbow.Rainbow_GetStreamClient

	clusterInfoChanged chan struct{}
}

func NewOutpost(ctx context.Context, cloudAddr string, w *watcher.Watcher) (*Outpost, error) {
	outpost := &Outpost{
		ctx:                ctx,
		cloudAddr:          cloudAddr,
		requestSeq:         10000,
		w:                  w,
		sendChan:           make(chan *rainbow.Content, 1),
		r:                  rainbow.NewRouter(),
		clusterInfoChanged: make(chan struct{}, 1),
	}

	// 注册回调函数
	err := w.SetOnInfoUpdate(infos.InfoType_CLUSTER_INFO,
		"outpost_cluster_listen_"+strconv.FormatUint(rand.Uint64()%100, 10),
		func(info infos.Information) {
			outpost.clusterInfoChanged <- struct{}{}
		})
	if err != nil {
		return nil, err
	}

	return outpost, nil
}

func (o *Outpost) SendChanListenLoop() {
	for {
		select {
		case <-o.ctx.Done():
			return
		case content := <-o.sendChan:
			// retry 3 times
			for i := 0; i < 3; i++ {
				err := o.stream.Send(content)
				if err == nil {
					break
				}
			}
		}
	}
}

func (o *Outpost) Send(request *rainbow.Request) <-chan *rainbow.Response {
	request.RequestSeq = o.requestSeq
	o.requestSeq += 1
	respChan := o.r.Register(request.RequestSeq)
	o.sendChan <- &rainbow.Content{
		Payload: &rainbow.Content_Request{
			Request: request,
		},
	}
	return respChan
}

func (o *Outpost) reportLoop() {
	for {
		select {
		case <-o.ctx.Done():
			return
		case <-o.clusterInfoChanged:
			if !o.w.GetMoon().IsLeader() {
				continue
			}
			clusterInfo := o.w.GetCurrentClusterInfo()
			o.Send(&rainbow.Request{
				RequestSeq: 0,
				Method:     rainbow.Request_PUT,
				Resource:   rainbow.Request_INFO,
				Info:       clusterInfo.BaseInfo(),
			})
		}
	}
}

func (o *Outpost) streamLoop() (err error) {
	for {
		select {
		case <-o.ctx.Done():
			return err
		default:
		}
		content, err := o.stream.Recv()
		if err != nil {
			return err
		}
		switch content.GetPayload().(type) {
		case *rainbow.Content_Request:
			request := content.GetRequest()
			switch request.GetResource() {
			case rainbow.Request_META:
				go o.doMetaRequest(request)
			default:
				logger.Errorf("unknown request resource: %v", request.GetResource())
			}
		case *rainbow.Content_Response:
			response := content.GetResponse()
			o.r.Send(response.ResponseTo, response)
		}
	}
}

func (o *Outpost) Run() error {
	// 等待集群信息就绪
	ok := o.w.WaitClusterOK()
	if !ok {
		logger.Errorf("cluster is not ok, outpost exit")
		return errors.New("cluster is not ok")
	}

	// 与云端建立连接
	port, _ := strconv.Atoi(strings.Split(o.cloudAddr, ":")[1])
	addr := strings.Split(o.cloudAddr, ":")[0]
	conn, err := messenger.GetRpcConn(addr, uint64(port))
	if err != nil {
		return err
	}

	rainbowClient := rainbow.NewRainbowClient(conn)
	stream, err := rainbowClient.GetStream(context.Background())
	if err != nil {
		return err
	}
	o.stream = stream

	// 初始化发送自身信息
	err = stream.Send(&rainbow.Content{
		Payload: &rainbow.Content_Request{
			Request: &rainbow.Request{
				RequestSeq: o.requestSeq,
				Method:     rainbow.Request_PUT,
				Resource:   rainbow.Request_INFO,
				Info:       o.w.GetSelfInfo().BaseInfo(),
			},
		},
	})
	o.requestSeq += 1
	if err != nil {
		return err
	}

	go o.SendChanListenLoop()
	go o.reportLoop()

	return o.streamLoop()
}

func (o *Outpost) doMetaRequest(request *rainbow.Request) {
	switch request.GetMethod() {
	case rainbow.Request_LIST:
		clientConfig := config.DefaultConfig // TODO 个性化配置
		clientConfig.NodeAddr = "127.0.0.1"
		clientConfig.NodePort = o.w.GetSelfInfo().RpcPort
		c, err := client.New(&clientConfig)
		metas, err := c.ListObjects(o.ctx, "default", request.RequestId)

		if err != nil {
			o.sendChan <- &rainbow.Content{
				Payload: &rainbow.Content_Response{
					Response: &rainbow.Response{
						ResponseTo: request.RequestSeq,
						Result: &common.Result{
							Status:  common.Result_FAIL,
							Message: err.Error(),
						},
					},
				},
			}
			return
		}

		o.sendChan <- &rainbow.Content{
			Payload: &rainbow.Content_Response{
				Response: &rainbow.Response{
					ResponseTo: request.RequestSeq,
					Result: &common.Result{
						Status: common.Result_OK,
					},
					Metas:  metas,
					IsLast: true,
				},
			},
		}
		return
	default:
		o.sendChan <- &rainbow.Content{
			Payload: &rainbow.Content_Response{
				Response: &rainbow.Response{
					ResponseTo: request.RequestSeq,
					Result: &common.Result{
						Status:  common.Result_FAIL,
						Message: "not support method",
					},
				},
			},
		}
		logger.Errorf("request method not support: %v", request.GetMethod())
	}
}
