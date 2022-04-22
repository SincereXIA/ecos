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

/* ALAYA error */
const (
	CodePgNotExist int32 = AlayaError + iota
	CodeMetaNotExist
	CodeMetaStorageNotExist
)

var (
	PGNotExist          = newErr(CodePgNotExist, "place group not exist")
	MetaNotExist        = newErr(CodeMetaNotExist, "meta data not exist")
	MetaStorageNotExist = newErr(CodeMetaStorageNotExist, "meta data storage not exist")
)

/* MOON error */
const (
	CodeConnectSunFail int32 = MoonError + iota
	CodeMoonRaftNotReady
	CodeInfoTypeNotSupport
	CodeInfoNotFound
	CodeTermNotMatch
	CodePgNotMatch
	CodeRaftNodeNotFound
)

var (
	ConnectSunFail     = newErr(CodeConnectSunFail, "connect sun fail")
	MoonRaftNotReady   = newErr(CodeMoonRaftNotReady, "moon raft not ready")
	InfoTypeNotSupport = newErr(CodeInfoTypeNotSupport, "info type not support")
	InfoNotFound       = newErr(CodeInfoNotFound, "info not found")
	TermNotMatch       = newErr(CodeTermNotMatch, "term not match")
	PgNotMatch         = newErr(CodePgNotMatch, "place group not match")
	RaftNodeNotFound   = newErr(CodeRaftNodeNotFound, "raft node not found")
)

const (
	// Client errors

	CodeIncompatibleSize = ClientError + iota

	CodeFullBuffer

	CodeIllegalStatus
	CodeRepeatedClose
)

var (
	IncompatibleSize = newErr(CodeIncompatibleSize, "incompatible size")

	IllegalStatus = newErr(CodeIllegalStatus, "block illegal status")
	RepeatedClose = newErr(CodeRepeatedClose, "block repeated close")
)

const (
	// Utils Common Error

	CodePoolClosed = CommonError + iota
	CodeZeroSize
)

var (
	PoolClosed = newErr(CodePoolClosed, "pool has closed")
	ZeroSize   = newErr(CodeZeroSize, "0 for uint size")
)

/* Gaia error */
const (
	CodeNoTransporter int32 = GaiaError + iota
	CodeTransporterWriteFail
	CodeGaiaClosed
	CodeRemoteGaiaFail
)

var (
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
