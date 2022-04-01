package infos

func (m *UserInfo) GetInfoType() InfoType {
	return InfoType_USER_INFO
}

func (m *UserInfo) BaseInfo() *BaseInfo {
	return &BaseInfo{Info: &BaseInfo_UserInfo{UserInfo: m}}
}

func (m *UserInfo) GetID() string {
	return m.UserId
}
