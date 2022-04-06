package credentials

type Credential struct {
	AccessKey string
	SecretKey string
}

func New(accessKey string, secretKey string) Credential {
	return Credential{
		AccessKey: accessKey,
		SecretKey: secretKey,
	}
}

func (c *Credential) GetUserID() string {
	return "root"
}
