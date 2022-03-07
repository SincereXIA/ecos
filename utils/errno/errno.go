package errno

import (
	"errors"
)

const (
	AlayaError int = (1 + iota) * 1000
	GaiaError
	MoonError
	SunError

	SystemError int = 77 * 1000
)

const (
	/* ALAYA error */

	CodePgNotExist int = AlayaError + iota
	CodeMetaNotExist
)

const (
	CodeConnectSunFail int = MoonError + iota
	CodeMoonRaftNotReady
)

var (
	PGNotExist   = newErr(CodePgNotExist, "place group not exist")
	MetaNotExist = newErr(CodePgNotExist, "meta data not exist")

	ConnectSunFail = newErr(CodeConnectSunFail, "connect sun fail")

	MoonRaftNotReady = newErr(CodeMoonRaftNotReady, "moon raft not ready")
)

type Errno struct {
	error
	Code    int
	Message string
}

func newErr(code int, name string) Errno {
	return Errno{
		error:   errors.New(name),
		Code:    code,
		Message: "",
	}
}
