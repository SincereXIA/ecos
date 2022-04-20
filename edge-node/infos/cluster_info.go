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

func (m *ClusterInfo) GetHealthNode() []*NodeInfo {
	var nodes []*NodeInfo
	for _, node := range m.NodesInfo {
		if node.State == NodeState_ONLINE {
			nodes = append(nodes, node)
		}
	}
	return nodes
}
