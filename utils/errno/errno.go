package errno

import (
	"errors"
)

const (
	AlayaError int32 = (1 + iota) * 1000
	GaiaError
	MoonError
	SunError

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

/* Gaia error */
const (
	CodeNoTransporter int32 = GaiaError + iota
	CodeTransporterWriteFail
	CodeGaiaClosed
)

var (
	PGNotExist   = newErr(CodePgNotExist, "place group not exist")
	MetaNotExist = newErr(CodePgNotExist, "meta data not exist")

	ConnectSunFail = newErr(CodeConnectSunFail, "connect sun fail")

	MoonRaftNotReady = newErr(CodeMoonRaftNotReady, "moon raft not ready")

	/* Gaia error */

	NoTransporterErr     = newErr(CodeNoTransporter, "no transporter available")
	TransporterWriteFail = newErr(CodeTransporterWriteFail, "transporter write fail")
	GaiaClosedErr        = newErr(CodeGaiaClosed, "gaia context done")
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
