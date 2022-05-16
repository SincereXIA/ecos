package errno

import (
	"ecos/utils/logger"
	"fmt"
	"runtime/debug"
)

const (
	AlayaError int32 = (1 + iota) * 1000
	GaiaError
	MoonError
	SunError
	ClientError
	GatewayError
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

	CodeMethodNotAllowed
	CodeInvalidArgument

	CodeJobNotExist
)

var (
	IncompatibleSize = newErr(CodeIncompatibleSize, "incompatible size")

	IllegalStatus = newErr(CodeIllegalStatus, "block illegal status")
	RepeatedClose = newErr(CodeRepeatedClose, "block repeated close")

	MethodNotAllowed = newErr(CodeMethodNotAllowed, "method not allowed")
	InvalidArgument  = newErr(CodeInvalidArgument, "invalid argument")

	JobNotExist = newErr(CodeJobNotExist, "job not exist")
)

const (
	// Gateway errors

	CodeGatewayNotFound = GatewayError + iota
	CodeNoSuchBucket
	CodeMissingBucketName
	CodeMissingKey
	CodeEmptyField

	CodeMissingUploadId
	CodeInvalidUploadId
	CodeInvalidPartId
	CodeFileTooLarge

	CodeEntityTooSmall
	CodeInvalidPart
	CodeInvalidPartOrder
	CodeNoSuchUpload

	CodeObjectNotFound
	CodeInvalidObjectState

	CodeSignatureDoesNotMatch
)

var (
	// GatewayNotFound = newErr(CodeGatewayNotFound, "gateway not found")

	NoSuchBucket    = newErr(CodeNoSuchBucket, "bucket not found")
	MissingBucket   = newErr(CodeMissingBucketName, "missing bucket name")
	MissingKey      = newErr(CodeMissingKey, "missing key")
	EmptyField      = newErr(CodeEmptyField, "empty field")
	MissingUploadId = newErr(CodeMissingUploadId, "missing upload id")
	InvalidUploadId = newErr(CodeInvalidUploadId, "invalid upload id")
	InvalidPartId   = newErr(CodeInvalidPartId, "invalid part id")
	FileTooLarge    = newErr(CodeFileTooLarge, "file too large")

	EntityTooSmall   = newErr(CodeEntityTooSmall, "entity too small")
	InvalidPart      = newErr(CodeInvalidPart, "invalid part")
	InvalidPartOrder = newErr(CodeInvalidPartOrder, "invalid part order")
	NoSuchUpload     = newErr(CodeNoSuchUpload, "no such upload")

	ObjectNotFound     = newErr(CodeObjectNotFound, "object not found")
	InvalidObjectState = newErr(CodeInvalidObjectState, "invalid object state")

	SignatureDoesNotMatch = newErr(CodeSignatureDoesNotMatch, "signature does not match")
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
	Code       int32
	Message    string
	StackTrace string
}

func newErr(code int32, name string) Errno {
	return Errno{
		error:      nil,
		Code:       code,
		Message:    name,
		StackTrace: string(debug.Stack()),
	}
}

func (e Errno) Error() string {
	var s string
	if e.error != nil {
		s = fmt.Sprintf("[%d] %s, raw: %s", e.Code, e.Message, e.error.Error())
	} else {
		s = fmt.Sprintf("[%d] %s", e.Code, e.Message)
	}
	return s
}

func WrapError(err error, code int32, message string) Errno {
	return Errno{
		error:      err,
		Code:       code,
		Message:    message,
		StackTrace: string(debug.Stack()),
	}
}

func HandleError(err error) {
	if err == nil {
		return
	}
	logger.Errorf("%s", err.Error())
	if e, ok := err.(Errno); ok {
		fmt.Println("Raw error stack:")
		fmt.Println(e.StackTrace)
	} else {
		logger.Errorf("There was an unexpected issue; please report this as a bug")
	}
}
