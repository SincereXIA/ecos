package object

import (
	"ecos/client/config"
	"ecos/edge-node/alaya"
	"ecos/edge-node/gaia"
	"ecos/edge-node/moon"
	"ecos/edge-node/node"
	"ecos/messenger"
	"ecos/utils/common"
	"ecos/utils/timestamp"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"path"
	"runtime"
	"strconv"
	"testing"
	"time"
)

func TestEcosWriter(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	t.Logf("Current test filename: %s", filename)
	type args struct {
		objectSize int
		key        string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{"writer 8M object",
			args{
				1024 * 1024 * 8, // 8M
				"/path/8M_obj",
			},
			false,
		},
		{"writer 8.1M object",
			args{
				1024*1024*8 + 1024*100, // 8.1M
				"/path/8.1M_obj",
			},
			false,
		},
		{"writer 128M object",
			args{
				1024 * 1024 * 128, // 128M
				"/path/128M_obj",
			},
			false,
		},
	}
	basePath := "./ecos-data/"
	_ = common.InitAndClearPath(basePath)
	infos, moons, alayas, rpcServers, err := createServers(9, "", path.Join(basePath, "db", "moon"))
	if err != nil {
		t.Errorf("RpcServer error = %v", err)
	}
	for i := 0; i < 9; i++ {
		infoStorage := moons[i].InfoStorage
		gaia.NewGaia(rpcServers[i], infos[i], infoStorage,
			&gaia.Config{BasePath: path.Join(basePath, "gaia", strconv.Itoa(i+1))})
		i := i
		go func(rpc *messenger.RpcServer) {
			err := rpc.Run()
			if err != nil {
				t.Errorf("GaiaServer error = %v", err)
			}
		}(rpcServers[i])
		go moons[i].Run()
		go alayas[i].Run()
	}

	t.Cleanup(func() {
		for i := 0; i < 9; i++ {
			moons[i].Stop()
			alayas[i].Stop()
			rpcServers[i].Stop()
		}
	})

	time.Sleep(15 * time.Second) // ensure rpcServer running

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := config.DefaultConfig
			conf.NodeAddr = moons[0].SelfInfo.IpAddr
			conf.NodePort = moons[0].SelfInfo.RpcPort
			factory := NewEcosWriterFactory(conf)
			writer := factory.GetEcosWriter(tt.args.key)
			data := genTestData(tt.args.objectSize)
			writeSize, err := writer.Write(data)
			assert.NoError(t, err)
			assert.Equal(t, tt.args.objectSize, writeSize)
			assert.NoError(t, writer.Close())
			t.Logf("Upload Finish!")
			if (err != nil) != tt.wantErr {
				t.Errorf("PutObject() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func genTestData(size int) []byte {
	directSize := 1024 * 1024 * 10
	if size < directSize {
		data := make([]byte, size)
		rand.Read(data)
		return data
	}
	d := make([]byte, directSize)
	data := make([]byte, 0, size)
	for size-directSize > 0 {
		data = append(data, d...)
		size = size - directSize
	}
	data = append(data, d[0:size]...)
	return data
}

func createServers(num int, sunAddr string, basePath string) ([]*node.NodeInfo,
	[]*moon.Moon, []*alaya.Alaya, []*messenger.RpcServer, error) {
	err := common.InitAndClearPath(basePath)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	var infoStorages []node.InfoStorage
	var stableStorages []moon.Storage
	var rpcServers []*messenger.RpcServer
	var moons []*moon.Moon
	var nodeInfos []*node.NodeInfo
	var alayas []*alaya.Alaya

	for i := 0; i < num; i++ {
		raftID := uint64(i + 1)
		infoStorages = append(infoStorages, node.NewMemoryNodeInfoStorage())
		stableStorages = append(stableStorages, moon.NewStorage(path.Join(basePath, "/raft", strconv.Itoa(i+1))))
		port, rpcServer := messenger.NewRandomPortRpcServer()
		rpcServers = append(rpcServers, rpcServer)
		nodeInfos = append(nodeInfos, node.NewSelfInfo(raftID, "127.0.0.1", port))
		alayas = append(alayas, alaya.NewAlaya(nodeInfos[i], infoStorages[i], alaya.NewMemoryMetaStorage(), rpcServers[i]))
	}

	moonConfig := moon.DefaultConfig
	moonConfig.SunAddr = sunAddr
	moonConfig.GroupInfo = node.GroupInfo{
		Term:            0,
		LeaderInfo:      nil,
		UpdateTimestamp: timestamp.Now(),
	}

	for i := 0; i < num; i++ {
		if sunAddr != "" {
			moons = append(moons, moon.NewMoon(nodeInfos[i], moonConfig, rpcServers[i], infoStorages[i],
				stableStorages[i]))
		} else {
			moonConfig.GroupInfo.NodesInfo = nodeInfos
			moons = append(moons, moon.NewMoon(nodeInfos[i], moonConfig, rpcServers[i], infoStorages[i],
				stableStorages[i]))
		}
	}
	return nodeInfos, moons, alayas, rpcServers, nil
}
