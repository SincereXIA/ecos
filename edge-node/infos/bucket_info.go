package infos

import "path"

func (m *BucketInfo) GetInfoType() InfoType {
	return InfoType_BUCKET_INFO
}

func (m *BucketInfo) BaseInfo() *BaseInfo {
	return &BaseInfo{Info: &BaseInfo_BucketInfo{BucketInfo: m}}
}

func (m *BucketInfo) GetID() string {
	return path.Join(m.VolumeId, m.BucketId)
}
