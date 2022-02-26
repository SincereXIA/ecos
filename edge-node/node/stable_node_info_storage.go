package node

import (
	"ecos/utils/logger"
	"encoding/json"
	"errors"
	"github.com/mohae/deepcopy"
	"github.com/tecbot/gorocksdb"
	"strconv"
	"time"
)

const term = "Term"

var ( // rocksdb的设置参数
	opts         = gorocksdb.NewDefaultOptions()
	readOptions  = gorocksdb.NewDefaultReadOptions()
	writeOptions = gorocksdb.NewDefaultWriteOptions()
)

type StableNodeInfoStorage struct {
	db                   *gorocksdb.DB
	nowInfoMap           map[ID]NodeInfo
	nowGroupInfo         *GroupInfo
	uncommittedGroupInfo *GroupInfo
}

func (storage *StableNodeInfoStorage) UpdateNodeInfo(info *NodeInfo) error {
	nodeId := strconv.FormatUint(info.RaftId, 10)
	nodeInfoData, err := json.Marshal(info)
	if err != nil {
		return nil
	}
	err = storage.db.Put(writeOptions, []byte(nodeId), nodeInfoData)
	if err != nil {
		return err
	}
	storage.updateTimestamp()
	return nil
}

func (storage *StableNodeInfoStorage) DeleteNodeInfo(nodeId ID) error {
	id := strconv.FormatUint(uint64(nodeId), 10)
	err := storage.db.Delete(writeOptions, []byte(id))
	if err != nil {
		return err
	}
	storage.updateTimestamp()
	return nil
}

func (storage *StableNodeInfoStorage) GetNodeInfo(nodeId ID) (*NodeInfo, error) {
	readOptions := gorocksdb.NewDefaultReadOptions()
	id := strconv.FormatUint(uint64(nodeId), 10)
	nodeInfoData, err := storage.db.Get(readOptions, []byte(id))
	defer nodeInfoData.Free()
	if err != nil {
		return nil, errors.New("node not found")
	}
	nodeInfo := NodeInfo{}
	err = json.Unmarshal(nodeInfoData.Data(), &nodeInfo)
	if err != nil {
		return nil, err
	}
	return &nodeInfo, nil
}

func (storage *StableNodeInfoStorage) ListAllNodeInfo() map[ID]NodeInfo {
	NodeInfoMap := make(map[ID]NodeInfo)
	iterator := storage.db.NewIterator(readOptions) //迭代器，遍历数据库
	defer iterator.Close()
	iterator.SeekToFirst()
	for it := iterator; it.Valid(); it.Next() {
		key := it.Key()
		keyData, err := strconv.ParseUint(string(key.Data()), 10, 64)
		if err != nil {
			return nil
		}
		value := it.Value()
		valueData := value.Data()
		nodeinfo := NodeInfo{}
		err = json.Unmarshal(valueData, &nodeinfo)
		if err != nil {
			return nil
		}
		NodeInfoMap[ID(keyData)] = nodeinfo
		key.Free()
		value.Free()
	}
	return NodeInfoMap
}

func (storage *StableNodeInfoStorage) Commit() {
	storage.nowInfoMap = storage.ListAllNodeInfo()
	cpy := deepcopy.Copy(storage.uncommittedGroupInfo)
	storage.nowGroupInfo = cpy.(*GroupInfo)
	storage.nowGroupInfo.NodesInfo = map2Slice(storage.nowInfoMap)
	oldTerm := storage.uncommittedGroupInfo.Term
	storage.uncommittedGroupInfo.Term = uint64(time.Now().UnixNano())
	// prevent commit too quick let term equal
	if storage.uncommittedGroupInfo.Term == oldTerm {
		storage.uncommittedGroupInfo.Term += 1
		termData, err := json.Marshal(storage.uncommittedGroupInfo.Term)
		if err != nil {
			logger.Errorf("Marshal Term failed, err:%v\n", err)
		}
		err = storage.db.Put(writeOptions, []byte(term), termData)
		if err != nil {
			logger.Errorf("save Term failed, err:%v\n", err)
		}
	}
}

func (storage *StableNodeInfoStorage) GetGroupInfo() *GroupInfo {
	return storage.nowGroupInfo
}

func (storage *StableNodeInfoStorage) SetLeader(nodeId ID) error {

	info, err := storage.GetNodeInfo(nodeId)
	if err != nil {
		return errors.New("leader not found")
	}
	storage.uncommittedGroupInfo.LeaderInfo = info
	storage.updateTimestamp()

	return nil
}

func (storage *StableNodeInfoStorage) updateTimestamp() {
	storage.uncommittedGroupInfo.UpdateTimestamp = uint64(time.Now().Unix())
}

func (storage *StableNodeInfoStorage) Close() {
	storage.db.Close()
}

func NewStableNodeInfoStorage(dataBaseDir string) *StableNodeInfoStorage {
	opts.SetCreateIfMissing(true)
	db, err := gorocksdb.OpenDb(opts, dataBaseDir)
	if err != nil {
		logger.Errorf("open database failed, err:%v\n", err)
		return nil
	}
	return &StableNodeInfoStorage{
		db:         db,
		nowInfoMap: make(map[ID]NodeInfo),
		nowGroupInfo: &GroupInfo{
			Term:            0,
			LeaderInfo:      nil,
			NodesInfo:       nil,
			UpdateTimestamp: uint64(time.Now().UnixNano()),
		},
		uncommittedGroupInfo: &GroupInfo{
			Term:            uint64(time.Now().UnixNano()),
			LeaderInfo:      nil,
			NodesInfo:       nil,
			UpdateTimestamp: uint64(time.Now().UnixNano()),
		},
	}
}
