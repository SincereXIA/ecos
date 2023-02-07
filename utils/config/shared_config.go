package config

const BehaveEcos = "ecos"
const BehaveCeph = "ceph"
const BehaveMapX = "mapx"

type SharedConfig struct {
	// exp only
	Behave string
}

var GlobalSharedConfig *SharedConfig

func init() {
	GlobalSharedConfig = &SharedConfig{
		Behave: BehaveEcos,
	}
}
