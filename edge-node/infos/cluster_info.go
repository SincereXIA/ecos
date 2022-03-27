package infos

import "strconv"

func (m *ClusterInfo) GetInfoType() InfoType {
	return InfoType_CLUSTER_INFO
}

func (m *ClusterInfo) BaseInfo() *BaseInfo {
	return &BaseInfo{Info: &BaseInfo_ClusterInfo{ClusterInfo: m}}
}

func (m *ClusterInfo) GetID() string {
	return strconv.FormatUint(m.Term, 10)
}
