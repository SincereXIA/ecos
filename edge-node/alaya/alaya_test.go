package alaya

import (
	"context"
	"ecos/edge-node/infos"
	"ecos/edge-node/moon"
	"ecos/edge-node/object"
	"ecos/edge-node/pipeline"
	"ecos/edge-node/watcher"
	"ecos/messenger"
	"ecos/utils/common"
	"ecos/utils/logger"
	"ecos/utils/timestamp"
	"github.com/stretchr/testify/assert"
	"os"
	"path"
	"strconv"
	"testing"
	"time"
)

func TestNewAlaya(t *testing.T) {
	basePath := "./ecos-data/"
	_ = common.InitAndClearPath(basePath)
	ctx := context.Background()
	//infoStorage := infos.NewStableNodeInfoStorage(nodeInfoDir)

	nodeNum := 9

	watchers, rpcServers, sunAddr := watcher.GenTestWatcherCluster(ctx, basePath, nodeNum)

	alayas := GenAlayaCluster(ctx, basePath, watchers, rpcServers)

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
		for i := 0; i < nodeNum; i++ { // for each node
			alaya := alayas[i]
			alaya.Stop()
			server := rpcServers[i]
			server.Stop()
		}

		_ = os.RemoveAll(basePath)
	})

	t.Log("Alayas init done, start run")
	waiteAllAlayaOK(alayas)
	pipelines := pipeline.GenPipelines(watchers[0].GetCurrentClusterInfo(), 10, 3)
	assertAlayasOK(t, alayas, pipelines)

	for i := 0; i < nodeNum; i++ { // for each node
		a := alayas[i]
		a.PrintPipelineInfo()
	}

	bucketInfo := infos.GenBucketInfo("root", "default", "root")
	_, err := watchers[0].GetMoon().ProposeInfo(ctx, &moon.ProposeInfoRequest{
		Operate:  moon.ProposeInfoRequest_ADD,
		Id:       bucketInfo.GetID(),
		BaseInfo: bucketInfo.BaseInfo(),
	})
	assert.NoError(t, err)

	testMetaNum := 100

	t.Run("update objectMeta", func(t *testing.T) {
		metas := genTestMetas(watchers, bucketInfo, testMetaNum)
		updateMetas(t, watchers, alayas, metas, bucketInfo)
		t.Run("list all objectMeta", func(t *testing.T) {
			var allMetas []*object.ObjectMeta
			for i := 1; i <= int(bucketInfo.Config.KeySlotNum); i++ {
				pgID := object.GenSlotPgID(bucketInfo.GetID(), int32(i), 10)
				nodeID := pipelines[pgID-1].RaftId[0]
				alaya := alayas[nodeID-1]
				ms, err := alaya.ListMeta(ctx, &ListMetaRequest{
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

	t.Run("add new nodes", func(t *testing.T) {
		var newWatchers []*watcher.Watcher
		var newRpcs []*messenger.RpcServer
		for i := 0; i < 3; i++ {
			newWatcher, newRpc := watcher.GenTestWatcher(ctx, path.Join(basePath, strconv.Itoa(nodeNum+i+1)), sunAddr)
			newWatchers = append(newWatchers, newWatcher)
			newRpcs = append(newRpcs, newRpc)
		}
		newAlayas := GenAlayaCluster(ctx, path.Join(basePath, "new"), newWatchers, newRpcs)
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
		waiteAllAlayaOK(alayas)
		pipelines = pipeline.GenPipelines(watchers[0].GetCurrentClusterInfo(), 10, 3)
	})

	assertAlayasOK(t, alayas, pipelines)

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
	alayas []*Alaya, metas []*object.ObjectMeta, bucketInfo *infos.BucketInfo) {
	p := pipeline.GenPipelines(watchers[0].GetCurrentClusterInfo(), 10, 3)
	for _, meta := range metas {
		_, _, key, _, err := object.SplitID(meta.ObjId)
		pgID := object.GenObjPgID(bucketInfo, key, 10)
		nodeIndex := p[pgID-1].RaftId[0]
		a := alayas[nodeIndex-1]
		logger.Debugf("Obj: %v, pgID: %v, nodeID: %v", meta.ObjId, pgID, nodeIndex)

		meta.PgId = pgID
		_, err = a.RecordObjectMeta(context.TODO(), meta)
		assert.NoError(t, err)
	}
}

func GenAlayaCluster(ctx context.Context, basePath string, watchers []*watcher.Watcher, rpcServers []*messenger.RpcServer) []*Alaya {
	var alayas []*Alaya
	nodeNum := len(watchers)
	for i := 0; i < nodeNum; i++ {
		// TODO (qiutb): apply stable meta storage
		//metaStorage := NewStableMetaStorage(path.Join(basePath, strconv.Itoa(i), "alaya", "meta"))
		metaStorage := NewMemoryMetaStorage()
		a := NewAlaya(ctx, watchers[i], metaStorage, rpcServers[i])
		alayas = append(alayas, a)
	}
	return alayas
}

func assertAlayasOK(t *testing.T, alayas []*Alaya, pipelines []*pipeline.Pipeline) {
	// 判断 每个 pg 第一个 节点是否为 leader
	for _, p := range pipelines {
		leaderID := p.RaftId[0]
		pgID := p.PgId
		a := alayas[leaderID-1]
		assert.Equal(t, leaderID, a.getRaftNode(pgID).raft.Status().Lead)
	}
	// 判断 每个 alaya 的每个 raft node 是否都成功加入 PG
	for _, a := range alayas {
		a.PGRaftNode.Range(func(key, value interface{}) bool {
			raftNode := value.(*Raft)
			assert.NotZero(t, raftNode.raft.Status().Lead)
			return true
		})
	}
}

func waiteAllAlayaOK(alayas []*Alaya) {
	timer := time.After(60 * time.Second)
	for {
		select {
		case <-timer:
			logger.Warningf("Alayas not OK after time out")
			for _, alaya := range alayas {
				alaya.PrintPipelineInfo()
			}
			return
		default:
		}
		ok := true
		for _, alaya := range alayas {
			if !alaya.IsAllPipelinesOK() {
				//logger.Warningf("Alaya %v not ok", id+1)
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
