package infos

import "ecos/utils/logger"

type Information interface {
	BaseInfo() *BaseInfo
	GetInfoType() InfoType
	GetID() string
}

func (m *BaseInfo) GetInfoType() InfoType {
	switch m.Info.(type) {
	case *BaseInfo_NodeInfo:
		return InfoType_NODE_INFO
	case *BaseInfo_ClusterInfo:
		return InfoType_CLUSTER_INFO
	case *BaseInfo_BucketInfo:
		return InfoType_BUCKET_INFO
	}
	logger.Errorf("get invalid info type")
	return InfoType_INVALID
}

func (m *BaseInfo) GetID() string {
	switch info := m.Info.(type) {
	case *BaseInfo_NodeInfo:
		return info.NodeInfo.GetID()
	case *BaseInfo_BucketInfo:
		return info.BucketInfo.GetID()
	case *BaseInfo_ClusterInfo:
		return info.ClusterInfo.GetID()
	}
	logger.Errorf("get invalid info type")
	return "INFO_TYPE_INVALID"
}

func (m *BaseInfo) BaseInfo() *BaseInfo {
	return m
}

type InvalidInfo struct {
}

func (i InvalidInfo) BaseInfo() *BaseInfo {
	return &BaseInfo{Info: &BaseInfo_ClusterInfo{}}
}

func (i InvalidInfo) GetInfoType() InfoType {
	return InfoType_INVALID
}

func (i InvalidInfo) GetID() string {
	return "INFO_TYPE_INVALID"
}
