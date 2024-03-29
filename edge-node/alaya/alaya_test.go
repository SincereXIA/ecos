package alaya

import (
	"context"
	"ecos/edge-node/infos"
	"ecos/edge-node/object"
	"ecos/edge-node/pipeline"
	"ecos/edge-node/watcher"
	"ecos/messenger"
	"ecos/shared/alaya"
	"ecos/shared/moon"
	"ecos/utils/common"
	"ecos/utils/logger"
	"ecos/utils/timestamp"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"os"
	"path"
	"strconv"
	"testing"
	"time"
)

func TestAlaya(t *testing.T) {
	t.Run("Real Alaya", func(t *testing.T) {
		testAlaya(t, false)
	})
	t.Run("Mock Alaya", func(t *testing.T) {
		testAlaya(t, true)
	})
}

func testAlaya(t *testing.T, mock bool) {
	basePath := "./ecos-data/"
	_ = common.InitAndClearPath(basePath)
	ctx, cancel := context.WithCancel(context.Background())

	nodeNum := 9
	var alayas []Alayaer
	watchers, rpcServers, sunAddr := watcher.GenTestWatcherCluster(ctx, basePath, nodeNum)
	mockMetaStorage := alaya.NewMemoryMetaStorage()
	if mock {
		alayas = GenMockAlayaCluster(t, ctx, basePath, mockMetaStorage, watchers, rpcServers)
	} else {
		alayas = GenAlayaCluster(ctx, basePath, watchers, rpcServers)
	}

	for _, rpc := range rpcServers {
		go func(r *messenger.RpcServer) {
			err := r.Run()
			if err != nil {
				t.Errorf("rpc server run err: %v", err)
			}
		}(rpc)
	}
	time.Sleep(time.Millisecond * 100)

	watcher.RunAllTestWatcher(watchers)
	for i := 0; i < nodeNum; i++ {
		a := alayas[i]
		go a.Run()
	}

	t.Cleanup(func() {
		logger.Infof("cleanup")
		for _, a := range alayas { // stop alaya first to avoid panic
			a.Stop()
		}
		time.Sleep(time.Second)
		cancel()
		for i := 0; i < nodeNum; i++ { // for each node
			server := rpcServers[i]
			server.Stop()
		}
		_ = os.RemoveAll(basePath)
		logger.Infof("cleanup done")
		time.Sleep(1 * time.Second)
	})

	t.Log("Alayas init done, start run")
	watcher.WaitAllTestWatcherOK(watchers)
	waiteAllAlayaOK(alayas)
	pipelines := pipeline.GenPipelines(watchers[0].GetCurrentClusterInfo(), uint64(watchers[0].GetCurrentClusterInfo().MetaPgNum),
		uint64(watchers[0].GetCurrentClusterInfo().MetaPgSize))
	assertAlayasOK(t, alayas, pipelines)

	bucketInfo := infos.GenBucketInfo("root", "default", "root")
	_, err := watchers[0].GetMoon().ProposeInfo(ctx, &moon.ProposeInfoRequest{
		Operate:  moon.ProposeInfoRequest_ADD,
		Id:       bucketInfo.GetID(),
		BaseInfo: bucketInfo.BaseInfo(),
	})
	assert.NoError(t, err)

	testMetaNum := 100

	var metas []*object.ObjectMeta

	t.Run("update objectMeta", func(t *testing.T) {
		metas = genTestMetas(watchers, bucketInfo, testMetaNum)
		updateMetas(t, watchers, alayas, metas, bucketInfo)
		t.Run("list all objectMeta", func(t *testing.T) {
			var allMetas []*object.ObjectMeta
			for i := 1; i <= int(bucketInfo.Config.KeySlotNum); i++ {
				pgID := object.GenSlotPgID(bucketInfo.GetID(), int32(i), watchers[0].GetCurrentClusterInfo().MetaPgNum)
				nodeID := pipelines[pgID-1].RaftId[0]
				a := alayas[nodeID-1]
				ctx, _ = alaya.SetTermToIncomingContext(ctx, watchers[0].GetCurrentTerm())
				ms, err := a.ListMeta(ctx, &alaya.ListMetaRequest{
					Prefix: path.Join(bucketInfo.GetID(), strconv.Itoa(i)),
				})
				if err != nil {
					t.Errorf("list meta err: %v", err)
				}
				allMetas = append(allMetas, ms.Metas...)
			}
			assert.Equal(t, testMetaNum, len(allMetas))
		})
	})

	var newWatchers []*watcher.Watcher
	var newRpcs []*messenger.RpcServer
	var newAlayas []Alayaer
	oldTerm := watchers[0].GetCurrentClusterInfo().Term
	t.Run("add new nodes", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			newWatcher, newRpc := watcher.GenTestWatcher(ctx, path.Join(basePath, strconv.Itoa(nodeNum+i)), sunAddr)
			newWatchers = append(newWatchers, newWatcher)
			newRpcs = append(newRpcs, newRpc)
		}
		if mock {
			newAlayas = GenMockAlayaCluster(t, ctx, path.Join(basePath, "new"), mockMetaStorage, newWatchers, newRpcs)
		} else {
			newAlayas = GenAlayaCluster(ctx, path.Join(basePath, "new"), newWatchers, newRpcs)
		}
		for _, rpc := range newRpcs {
			go func(r *messenger.RpcServer) {
				err := r.Run()
				if err != nil {
					t.Errorf("rpc server run err: %v", err)
				}
			}(rpc)
		}
		for _, a := range newAlayas {
			go a.Run()
		}
		watcher.RunAllTestWatcher(newWatchers)
		alayas = append(alayas, newAlayas...)
		watchers = append(watchers, newWatchers...)

		watcher.WaitAllTestWatcherOK(watchers)
		// wait until Term changed
		for watchers[0].GetCurrentClusterInfo().Term == oldTerm {
			time.Sleep(100 * time.Millisecond)
		}
		waiteAllAlayaOK(alayas)
		pipelines = pipeline.GenPipelines(watchers[0].GetCurrentClusterInfo(), uint64(watchers[0].GetCurrentClusterInfo().MetaPgNum),
			uint64(watchers[0].GetCurrentClusterInfo().MetaPgSize))
	})
	assertAlayasOK(t, alayas, pipelines)

	t.Cleanup(func() {
		for _, rpc := range newRpcs {
			rpc.Stop()
		}
	})

	t.Run("delete object meta", func(t *testing.T) {
		meta := metas[0]
		_, _, key, _, err := object.SplitID(meta.ObjId)
		if err != nil {
			t.Errorf("object id split err: %v", err)
		}
		pgID := object.GenObjPgID(bucketInfo, key, watchers[0].GetCurrentClusterInfo().MetaPgNum)
		nodeIndex := pipelines[pgID-1].RaftId[0]
		a := alayas[nodeIndex-1]
		ctx, _ = alaya.SetTermToIncomingContext(ctx, watchers[0].GetCurrentTerm())
		reply, err := a.GetObjectMeta(ctx, &alaya.MetaRequest{
			ObjId: meta.ObjId,
		})
		assert.NoError(t, err, "meta need to delete should exist")
		assert.Equal(t, meta.ObjId, reply.ObjId, "meta objID not equal")
		_, err = a.DeleteMeta(ctx, &alaya.DeleteMetaRequest{
			ObjId: meta.ObjId,
		})
		if err != nil {
			t.Errorf("delete object meta err: %v", err)
		}

		_, err = a.GetObjectMeta(ctx, &alaya.MetaRequest{
			ObjId: meta.ObjId,
		})
		assert.Error(t, err, "get not exist meta should return error")
	})

}

func genTestMetas(watchers []*watcher.Watcher,
	bucketInfo *infos.BucketInfo, num int) []*object.ObjectMeta {
	metas := make([]*object.ObjectMeta, num)
	for i := 0; i < num; i++ {
		key := "test" + strconv.Itoa(i)
		meta := &object.ObjectMeta{
			ObjId:      object.GenObjectId(bucketInfo, key),
			ObjSize:    100,
			UpdateTime: timestamp.Now(),
			ObjHash:    "",
			PgId:       0,
			Blocks:     nil,
			Term:       watchers[0].GetCurrentTerm(),
		}
		metas[i] = meta
	}
	return metas
}

func updateMetas(t *testing.T, watchers []*watcher.Watcher,
	alayas []Alayaer, metas []*object.ObjectMeta, bucketInfo *infos.BucketInfo) {
	clusterInfo := watchers[0].GetCurrentClusterInfo()
	clusterP, _ := pipeline.NewClusterPipelines(clusterInfo)
	p := clusterP.MetaPipelines
	ctx := context.Background()
	for _, meta := range metas {
		_, _, key, _, err := object.SplitID(meta.ObjId)
		pgID := object.GenObjPgID(bucketInfo, key, watchers[0].GetCurrentClusterInfo().MetaPgNum)
		nodeIndex := p[pgID-1].RaftId[0]
		a := alayas[nodeIndex-1]
		logger.Debugf("Obj: %v, pgID: %v, nodeID: %v", meta.ObjId, pgID, nodeIndex)

		meta.PgId = pgID
		ctx, _ = alaya.SetTermToIncomingContext(ctx, meta.Term)
		_, err = a.RecordObjectMeta(ctx, meta)
		assert.NoError(t, err)
	}
}

func GenAlayaCluster(ctx context.Context, basePath string, watchers []*watcher.Watcher, rpcServers []*messenger.RpcServer) []Alayaer {
	var alayas []Alayaer
	alayaConfig := DefaultConfig
	nodeNum := len(watchers)
	for i := 0; i < nodeNum; i++ {
		// TODO (qiutb): apply stable meta storage
		//metaStorageRegister := NewMemoryMetaStorageRegister()
		metaStorageRegister, err := NewRocksDBMetaStorageRegister(path.Join(basePath, strconv.FormatInt(int64(i), 10), "meta"))
		if err != nil {
			logger.Errorf("new meta storage register err: %v", err)
		}
		a := NewAlaya(ctx, watchers[i], &alayaConfig, metaStorageRegister, rpcServers[i])
		alayas = append(alayas, a)
	}
	return alayas
}

func GenMockAlayaCluster(t *testing.T, ctx context.Context, basePath string, storage alaya.MetaStorage,
	watchers []*watcher.Watcher, rpcServers []*messenger.RpcServer) []Alayaer {
	var alayas []Alayaer
	nodeNum := len(watchers)
	for i := 0; i < nodeNum; i++ {
		ctrl := gomock.NewController(t)
		alaya := NewMockAlayaer(ctrl)

		InitMock(alaya, rpcServers[i], storage)
		alayas = append(alayas, alaya)
	}
	return alayas
}

func assertAlayasOK(t *testing.T, alayas []Alayaer, pipelines []*pipeline.Pipeline) {
	// 判断 每个 pg 第一个 节点是否为 leader
	for _, p := range pipelines {
		leaderID := p.RaftId[0]
		pgID := p.PgId
		a := alayas[leaderID-1]
		switch x := a.(type) {
		case *Alaya:
			assert.Equal(t, leaderID, x.getRaftNode(pgID).raft.Node.Status().Lead)
		}
	}
	// 判断 每个 alaya 的每个 raft node 是否都成功加入 PG
	for _, a := range alayas {
		switch x := a.(type) {
		case *Alaya:
			x.PGRaftNode.Range(func(key, value interface{}) bool {
				raftNode := value.(*Raft)
				assert.NotZero(t, raftNode.raft.Node.Status().Lead)
				return true
			})
		}
	}
}

func waiteAllAlayaOK(alayas []Alayaer) {
	timer := time.After(60 * time.Second)
	for {
		select {
		case <-timer:
			logger.Warningf("Alayas not OK after time out")
			for _, a := range alayas {
				switch x := a.(type) {
				case *Alaya:
					x.PrintPipelineInfo()
				}
			}
			return
		default:
		}
		ok := true
		for _, alaya := range alayas {
			if !alaya.IsAllPipelinesOK() {
				ok = false
				break
			}
		}
		if ok {
			return
		}
		time.Sleep(time.Millisecond * 200)
	}
}
