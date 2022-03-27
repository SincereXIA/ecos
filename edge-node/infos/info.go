package infos

import (
	"errors"
	"strconv"
)

type Information struct {
	infoType InfoType
	id       string
	info     BaseInfo
}

func BaseInfoToInformation(baseInfo BaseInfo) (Information, error) {
	switch info := baseInfo.Info.(type) {
	case *BaseInfo_NodeInfo:
		return Information{
			infoType: InfoType_NODE_INFO,
			id:       strconv.FormatUint(info.NodeInfo.RaftId, 10),
			info:     baseInfo,
		}, nil
	case *BaseInfo_ClusterInfo:
		return Information{
			infoType: InfoType_CLUSTER_INFO,
			id:       strconv.FormatUint(info.ClusterInfo.Term, 10),
			info:     baseInfo,
		}, nil
	default:
		return Information{}, errors.New("data type not support")
	}
}

func (info Information) GetInfoType() InfoType {
	return info.infoType
}

func (info Information) BaseInfo() *BaseInfo {
	return &info.info
}
