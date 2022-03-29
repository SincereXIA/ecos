package infos

func (m *VolumeInfo) GetInfoType() InfoType {
	return InfoType_VOLUME_INFO
}

func (m *VolumeInfo) BaseInfo() *BaseInfo {
	return &BaseInfo{Info: &BaseInfo_VolumeInfo{VolumeInfo: m}}
}

func (m *VolumeInfo) GetID() string {
	return m.VolumeId
}
