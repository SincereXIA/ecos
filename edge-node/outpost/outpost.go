package outpost

import (
	"context"
	"ecos/client"
	"ecos/client/config"
	"ecos/cloud/rainbow"
	"ecos/edge-node/infos"
	"ecos/edge-node/object"
	"ecos/edge-node/pipeline"
	"ecos/edge-node/watcher"
	"ecos/messenger"
	"ecos/messenger/common"
	"ecos/shared/alaya"
	"ecos/shared/gaia"
	"ecos/utils/logger"
	"errors"
	"io"
	"math/rand"
	"strconv"
	"strings"
	"sync"
)

type Outpost struct {
	ctx context.Context

	cloudAddr  string
	requestSeq uint64

	w *watcher.Watcher
	r *rainbow.Router

	sendChan chan *rainbow.Content
	stream   rainbow.Rainbow_GetStreamClient

	mu sync.Mutex // protect requestSeq

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
			go func() {
				outpost.clusterInfoChanged <- struct{}{}
			}()
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
	o.mu.Lock()
	request.RequestSeq = o.requestSeq
	o.requestSeq += 1
	o.mu.Unlock()

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

// streamLoop 接受云端发送的结果和请求
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
		// 处理请求
		case *rainbow.Content_Request:
			request := content.GetRequest()
			switch request.GetResource() {
			case rainbow.Request_META:
				go o.doMetaRequest(request)
			case rainbow.Request_BLOCK:
				go o.doBlockRequest(request)
			case rainbow.Request_INFO:
				go o.doInfoRequest(request)
			default:
				logger.Errorf("unknown request resource: %v", request.GetResource())
			}
		// 处理响应
		case *rainbow.Content_Response:
			response := content.GetResponse()
			o.r.Send(response.ResponseTo, response)
			if response.IsLast {
				o.r.Unregister(response.ResponseTo)
			}
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
	if o.cloudAddr == "" {
		logger.Warningf("cloudAddr is empty, outpost exit")
		return errors.New("cloud address is empty")
	}

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
	_ = o.Send(&rainbow.Request{
		RequestSeq: o.requestSeq,
		Method:     rainbow.Request_PUT,
		Resource:   rainbow.Request_INFO,
		Info:       o.w.GetSelfInfo().BaseInfo(),
	})

	go o.SendChanListenLoop()
	go o.reportLoop()

	logger.Infof("outpost init success")

	return o.streamLoop()
}

func (o *Outpost) getClient() (*client.Client, error) {
	clientConfig := config.DefaultConfig // TODO 个性化配置
	clientConfig.NodeAddr = "127.0.0.1"
	clientConfig.NodePort = o.w.GetSelfInfo().RpcPort
	return client.New(&clientConfig)
}

func (o *Outpost) returnFail(request *rainbow.Request, err error) {
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
}

func (o *Outpost) doMetaRequest(request *rainbow.Request) {
	switch request.GetMethod() {
	case rainbow.Request_LIST:
		c, _ := o.getClient()
		_, bucketID, key, err := object.SplitPrefixWithoutSlotID(request.RequestId)
		bucketName := strings.Split(bucketID, "/")[2]
		metas, err := c.ListObjects(o.ctx, bucketName, key)

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
	case rainbow.Request_GET:
		c, _ := o.getClient()
		objID := request.RequestId
		_, bucketID, key, _, _ := object.SplitID(objID)
		split := strings.Split(bucketID, "/")
		bucketName := split[len(split)-1]
		bucket, _ := c.GetVolumeOperator().Get(bucketName)
		obj, err := bucket.Get(key)
		if err != nil {
			o.returnFail(request, err)
			break
		}
		meta := obj.(*client.ObjectOperator).Meta
		o.sendChan <- &rainbow.Content{
			Payload: &rainbow.Content_Response{
				Response: &rainbow.Response{
					ResponseTo: request.RequestSeq,
					Result: &common.Result{
						Status: common.Result_OK,
					},
					Metas: []*object.ObjectMeta{meta},
				},
			},
		}
	case rainbow.Request_PUT:
		meta := request.GetMeta()
		_, bucketID, key, _, _ := object.SplitID(meta.ObjId)
		bucketInfo, err := o.w.GetMoon().GetInfoDirect(infos.InfoType_BUCKET_INFO, bucketID)
		if err != nil {
			logger.Errorf("get bucket info failed, err: %v", err)
			o.returnFail(request, err)
			break
		}
		clusterInfo := o.w.GetCurrentClusterInfo()
		pipes, _ := pipeline.NewClusterPipelines(clusterInfo)
		pgID := object.GenObjPgID(bucketInfo.BaseInfo().GetBucketInfo(), key, clusterInfo.MetaPgNum)

		serverId := pipes.GetMetaPGNodeID(pgID)[0]

		nodeInfo, _ := o.w.GetMoon().GetInfoDirect(infos.InfoType_NODE_INFO, serverId)
		conn, err := messenger.GetRpcConnByNodeInfo(nodeInfo.BaseInfo().GetNodeInfo())
		if err != nil {
			logger.Errorf("get rpc conn failed, err: %v", err)
			o.returnFail(request, err)
			break
		}
		c := alaya.NewAlayaClient(conn)
		ctx, _ := alaya.SetTermToContext(o.ctx, o.w.GetCurrentTerm())
		_, err = c.RecordObjectMeta(ctx, meta)
		if err != nil {
			logger.Errorf("record object meta failed, err: %v", err)
			o.returnFail(request, err)
			break
		}
		o.sendChan <- &rainbow.Content{
			Payload: &rainbow.Content_Response{
				Response: &rainbow.Response{
					ResponseTo: request.RequestSeq,
					Result: &common.Result{
						Status: common.Result_OK,
					},
				},
			},
		}
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

func (o *Outpost) doBlockRequest(request *rainbow.Request) {
	switch request.GetMethod() {
	case rainbow.Request_GET:
		blockID := request.RequestId
		clusterInfo, err := o.w.GetClusterInfoByTerm(request.Term)
		if err != nil {
			o.returnFail(request, err)
			return
		}

		pgID := object.GenBlockPgID(blockID, clusterInfo.BlockPgNum)
		pipelines, err := pipeline.NewClusterPipelines(clusterInfo)
		if err != nil {
			o.returnFail(request, err)
			return
		}
		nodeIds := pipelines.GetBlockPG(pgID)

		// 连接所有可用节点
		for _, nodeID := range nodeIds {
			info, err := o.w.GetMoon().GetInfoDirect(infos.InfoType_NODE_INFO, strconv.FormatUint(nodeID, 10))
			if err != nil {
				continue
			}
			nodeInfo := info.BaseInfo().GetNodeInfo()
			conn, err := messenger.GetRpcConnByNodeInfo(nodeInfo)
			if err != nil {
				continue
			}

			c := gaia.NewGaiaClient(conn)
			req := &gaia.GetBlockRequest{
				BlockId:  blockID,
				CurChunk: 0,
				Term:     request.Term,
			}

			res, err := c.GetBlockData(o.ctx, req)
			if err != nil {
				logger.Warningf("blockClient responds err: %v", err)
				continue
			}
			startSend := false
			ok := true
			for {
				var rs *gaia.GetBlockResult
				rs, err = res.Recv() // 从流中接收数据
				if err != nil {
					if err == io.EOF {
						o.sendChan <- &rainbow.Content{
							Payload: &rainbow.Content_Response{
								Response: &rainbow.Response{
									ResponseTo: request.RequestSeq,
									Result: &common.Result{
										Status: common.Result_OK,
									},
									IsLast: true,
								},
							},
						}
						break
					}
					ok = false
					terr := res.CloseSend()
					if terr != nil {
						logger.Infof("close gaia server failed, err: %v", terr)
					}
					logger.Warningf("res.Recv err: %v", err)
					break
				}
				startSend = true // 有数据返回，开始发送
				o.sendChan <- &rainbow.Content{
					Payload: &rainbow.Content_Response{
						Response: &rainbow.Response{
							ResponseTo: request.RequestSeq,
							Result: &common.Result{
								Status: common.Result_OK,
							},
							Chunk: rs.GetChunk().Content,
						},
					},
				}
			}
			if ok {
				return
			}
			if startSend {
				o.returnFail(request, err)
			}
		}

		o.returnFail(request, errors.New("no available node"))
	default:
		logger.Errorf("request method not support: %v", request.GetMethod())
	}
}

func (o *Outpost) doInfoRequest(request *rainbow.Request) {
	switch request.GetMethod() {
	case rainbow.Request_GET:
		infoType := request.GetInfoType()
		infoID := request.RequestId
		logger.Debugf("outpost get info, type: %v, id: %v", infoType, infoID)
		var info infos.Information
		var err error
		if infoType == infos.InfoType_CLUSTER_INFO && infoID == "0" {
			clusterInfo := o.w.GetCurrentClusterInfo()
			info = clusterInfo.BaseInfo()
		} else {
			info, err = o.w.GetMoon().GetInfoDirect(infoType, infoID)
		}

		if err != nil {
			o.returnFail(request, err)
			return
		}
		o.sendChan <- &rainbow.Content{
			Payload: &rainbow.Content_Response{
				Response: &rainbow.Response{
					ResponseTo: request.RequestSeq,
					Result: &common.Result{
						Status: common.Result_OK,
					},
					Infos: []*infos.BaseInfo{info.BaseInfo()},
				},
			},
		}
	default:
		logger.Errorf("request method not support: %v", request.GetMethod())
	}
}
