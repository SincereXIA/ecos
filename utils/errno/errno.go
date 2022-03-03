package errno

import (
	"errors"
)

const (
	AlayaError int = (1 + iota) * 1000
	GaiaError
	MoonError
	SunError
	ClientError
	CommonError

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

const (
	// Client errors

	CodeIncompatibleSize = ClientError + iota

	CodeFullBuffer

	CodeIllegalStatus
	CodeRepeatedClose
)

const (
	// Utils Common Error

	CodePoolClosed = CommonError + iota
	CodeZeroSize
)

var (
	PGNotExist   = newErr(CodePgNotExist, "place group not exist")
	MetaNotExist = newErr(CodePgNotExist, "meta data not exist")

	ConnectSunFail = newErr(CodeConnectSunFail, "connect sun fail")

	MoonRaftNotReady = newErr(CodeMoonRaftNotReady, "moon raft not ready")
	// Client Upload Errors

	IncompatibleSize = newErr(CodeIncompatibleSize, "incompatible size")

	IllegalStatus = newErr(CodeIllegalStatus, "unit illegal status")
	RepeatedClose = newErr(CodeRepeatedClose, "unit repeated close")

	PoolClosed = newErr(CodePoolClosed, "pool has closed")
	ZeroSize   = newErr(CodeZeroSize, "0 for uint size")
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
