package infos

import (
	"errors"
	"strconv"
)

type Information struct {
	id   string
	info interface{}
}

func NewInformation(data interface{}) (Information, error) {
	switch info := data.(type) {
	case *NodeInfo:
		return Information{
			id:   strconv.FormatUint(info.RaftId, 10),
			info: *info,
		}, nil
	case *ClusterInfo:
		return Information{
			id:   strconv.FormatUint(info.Term, 10),
			info: *info,
		}, nil
	default:
		return Information{}, errors.New("data type not support")
	}
}
