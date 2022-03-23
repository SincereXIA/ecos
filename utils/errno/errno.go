package errno

import (
	"errors"
)

const (
	AlayaError int32 = (1 + iota) * 1000
	GaiaError
	MoonError
	SunError
	ClientError
	CommonError

	SystemError int32 = 77 * 1000
)

const (
	/* ALAYA error */

	CodePgNotExist int32 = AlayaError + iota
	CodeMetaNotExist
)

const (
	CodeConnectSunFail int32 = MoonError + iota
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

/* Gaia error */
const (
	CodeNoTransporter int32 = GaiaError + iota
	CodeTransporterWriteFail
	CodeGaiaClosed
	CodeRemoteGaiaFail
)

var (
	PGNotExist   = newErr(CodePgNotExist, "place group not exist")
	MetaNotExist = newErr(CodePgNotExist, "meta data not exist")

	ConnectSunFail = newErr(CodeConnectSunFail, "connect sun fail")

	MoonRaftNotReady = newErr(CodeMoonRaftNotReady, "moon raft not ready")
	// Client Upload Errors

	IncompatibleSize = newErr(CodeIncompatibleSize, "incompatible size")

	IllegalStatus = newErr(CodeIllegalStatus, "block illegal status")
	RepeatedClose = newErr(CodeRepeatedClose, "block repeated close")

	PoolClosed = newErr(CodePoolClosed, "pool has closed")
	ZeroSize   = newErr(CodeZeroSize, "0 for uint size")

	/* Gaia error */

	NoTransporterErr     = newErr(CodeNoTransporter, "no transporter available")
	TransporterWriteFail = newErr(CodeTransporterWriteFail, "transporter write fail")
	GaiaClosedErr        = newErr(CodeGaiaClosed, "gaia context done")
	RemoteGaiaFail       = newErr(CodeRemoteGaiaFail, "remote gaia fail")
)

type Errno struct {
	error
	Code    int32
	Message string
}

func newErr(code int32, name string) Errno {
	return Errno{
		error:   errors.New(name),
		Code:    code,
		Message: "",
	}
}
