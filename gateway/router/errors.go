package router

import (
	"ecos/utils/common"
	"fmt"
)

type Error struct {
	Code       *string `xml:"Code"`
	Message    *string `xml:"Message"`
	BucketName *string `xml:"BucketName"`
	Resource   *string `xml:"Resource"`
	Key        *string `xml:"Key"`
}

func (e Error) Error() string {
	return *e.Message
}

// BucketAlreadyExists 409
func BucketAlreadyExists(bucketName string) Error {
	return Error{
		Code:       common.PtrString("BucketAlreadyExists"),
		Message:    common.PtrString("The requested bucket name is not available. The bucket namespace is shared by all users of the system. Please select a different name and try again."),
		BucketName: common.PtrString(bucketName),
	}
}

// BucketAlreadyOwnedByYou 409
func BucketAlreadyOwnedByYou(bucketName string) Error {
	return Error{
		Code:       common.PtrString("BucketAlreadyOwnedByYou"),
		Message:    common.PtrString("The bucket you tried to create already exists, and you own it."),
		BucketName: common.PtrString(bucketName),
	}
}

// BucketNotEmpty 409
func BucketNotEmpty(bucketName string) Error {
	return Error{
		Code:       common.PtrString("BucketNotEmpty"),
		Message:    common.PtrString("The bucket you tried to delete is not empty."),
		BucketName: common.PtrString(bucketName),
	}
}

// EntityTooSmall 400
func EntityTooSmall(bucketName, resPath, key string) Error {
	return Error{
		Code:       common.PtrString("EntityTooSmall"),
		Message:    common.PtrString("Your proposed upload is smaller than the minimum allowed object size."),
		BucketName: common.PtrString(bucketName),
		Resource:   common.PtrString(resPath),
		Key:        common.PtrString(key),
	}
}

// EntityTooLarge 400
func EntityTooLarge(bucketName, resPath, key string) Error {
	return Error{
		Code:       common.PtrString("EntityTooLarge"),
		Message:    common.PtrString("Your proposed upload exceeds the maximum allowed object size."),
		BucketName: common.PtrString(bucketName),
		Resource:   common.PtrString(resPath),
		Key:        common.PtrString(key),
	}
}

// IncompleteBody 400
func IncompleteBody(bucketName, resPath, key string) Error {
	return Error{
		Code:       common.PtrString("IncompleteBody"),
		Message:    common.PtrString("Your proposed upload is too small."),
		BucketName: common.PtrString(bucketName),
		Resource:   common.PtrString(resPath),
		Key:        common.PtrString(key),
	}
}

// IncorrectNumberOfFilesInPostRequest 400
func IncorrectNumberOfFilesInPostRequest(bucketName, resPath string, key *string) Error {
	return Error{
		Code:       common.PtrString("IncorrectNumberOfFilesInPostRequest"),
		Message:    common.PtrString("Too many files in the request."),
		BucketName: common.PtrString(bucketName),
		Resource:   common.PtrString(resPath),
		Key:        key,
	}
}

// InternalError 500
func InternalError(message, bucketName, resPath string, key *string) Error {
	return Error{
		Code:       common.PtrString("InternalError"),
		Message:    common.PtrString(message),
		BucketName: common.PtrString(bucketName),
		Resource:   common.PtrString(resPath),
		Key:        key,
	}
}

// InvalidArgument 400
func InvalidArgument(keyName, bucketName, resPath string, key *string) Error {
	return Error{
		Code:       common.PtrString("InvalidArgument"),
		Message:    common.PtrString(fmt.Sprint("The argument you provided is invalid: ", keyName)),
		BucketName: common.PtrString(bucketName),
		Resource:   common.PtrString(resPath),
		Key:        key,
	}
}

// InvalidBucketName 400
func InvalidBucketName(bucketName *string) Error {
	return Error{
		Code:       common.PtrString("InvalidBucketName"),
		Message:    common.PtrString(fmt.Sprintf("The specified bucket is not valid.")),
		BucketName: bucketName,
	}
}

// InvalidObjectState 400
func InvalidObjectState(bucketName, resPath, key string) Error {
	return Error{
		Code:       common.PtrString("InvalidObjectState"),
		Message:    common.PtrString("The specified object is in a state that can't be transitioned to."),
		BucketName: common.PtrString(bucketName),
		Resource:   common.PtrString(resPath),
		Key:        common.PtrString(key),
	}
}

// InvalidPart 400
func InvalidPart(bucketName, resPath, key string) Error {
	return Error{
		Code:       common.PtrString("InvalidPart"),
		Message:    common.PtrString("One or more of the specified parts could not be found. The part might not have been uploaded, or the specified entity tag might not have matched the part's entity tag."),
		BucketName: common.PtrString(bucketName),
		Resource:   common.PtrString(resPath),
		Key:        common.PtrString(key),
	}
}

// InvalidPartOrder 400
func InvalidPartOrder(bucketName, resPath, key string) Error {
	return Error{
		Code:       common.PtrString("InvalidPartOrder"),
		Message:    common.PtrString("The list of parts was not in ascending order. The parts list must be specified in order by part number."),
		BucketName: common.PtrString(bucketName),
		Resource:   common.PtrString(resPath),
		Key:        common.PtrString(key),
	}
}

// InvalidRange 416
func InvalidRange(bucketName, resPath, key string) Error {
	return Error{
		Code:       common.PtrString("InvalidRange"),
		Message:    common.PtrString("The requested range cannot be satisfied."),
		BucketName: common.PtrString(bucketName),
		Resource:   common.PtrString(resPath),
		Key:        common.PtrString(key),
	}
}

// InvalidURI 400
func InvalidURI(bucketName, resPath string, key *string) Error {
	return Error{
		Code:       common.PtrString("InvalidURI"),
		Message:    common.PtrString("The specified URI is invalid."),
		BucketName: common.PtrString(bucketName),
		Resource:   common.PtrString(resPath),
		Key:        key,
	}
}

// KeyTooLong 400
func KeyTooLong(bucketName, resPath, key string) Error {
	return Error{
		Code:       common.PtrString("KeyTooLongError"),
		Message:    common.PtrString("Your key is too long."),
		BucketName: common.PtrString(bucketName),
		Resource:   common.PtrString(resPath),
		Key:        common.PtrString(key),
	}
}

// MalformedPOSTRequest 404
func MalformedPOSTRequest(bucketName, resPath string, key *string) Error {
	return Error{
		Code:       common.PtrString("MalformedPOSTRequest"),
		Message:    common.PtrString("The body of your POST request is not well-formed multipart/form-data."),
		BucketName: common.PtrString(bucketName),
		Resource:   common.PtrString(resPath),
		Key:        key,
	}
}

// MalformedXML 400
func MalformedXML(bucketName, resPath string, key *string) Error {
	return Error{
		Code:       common.PtrString("MalformedXML"),
		Message:    common.PtrString("The XML you provided was not well-formed or did not validate against our published schema."),
		BucketName: common.PtrString(bucketName),
		Resource:   common.PtrString(resPath),
		Key:        key,
	}
}

// MaxMessageLengthExceeded 400
func MaxMessageLengthExceeded(bucketName, resPath string, key *string) Error {
	return Error{
		Code:       common.PtrString("MaxMessageLengthExceeded"),
		Message:    common.PtrString("Your request was too big."),
		BucketName: common.PtrString(bucketName),
		Resource:   common.PtrString(resPath),
		Key:        key,
	}
}

// MetadataTooLarge 400
func MetadataTooLarge(bucketName, resPath string, key *string) Error {
	return Error{
		Code:       common.PtrString("MetadataTooLarge"),
		Message:    common.PtrString("Your metadata headers exceed the maximum allowed metadata size."),
		BucketName: common.PtrString(bucketName),
		Resource:   common.PtrString(resPath),
		Key:        key,
	}
}

// MethodNotAllowed 405
func MethodNotAllowed(bucketName, resPath string, key *string) Error {
	return Error{
		Code:       common.PtrString("MethodNotAllowed"),
		Message:    common.PtrString("The specified method is not allowed against this resource."),
		BucketName: common.PtrString(bucketName),
		Resource:   common.PtrString(resPath),
		Key:        key,
	}
}

// MissingRequestBody 411

// NoSuchBucket 404
func NoSuchBucket(bucketName string) Error {
	return Error{
		Code:       common.PtrString("NoSuchBucket"),
		Message:    common.PtrString("The specified bucket does not exist"),
		BucketName: common.PtrString(bucketName),
		Resource:   common.PtrString("/" + bucketName),
	}
}

// NoSuchKey 404
func NoSuchKey(bucketName, resPath, key string) Error {
	return Error{
		Code:       common.PtrString("NoSuchKey"),
		Message:    common.PtrString("The specified key does not exist."),
		BucketName: common.PtrString(bucketName),
		Resource:   common.PtrString(resPath),
		Key:        common.PtrString(key),
	}
}

// NoSuchUpload 404
func NoSuchUpload(bucketName, resPath, key string) Error {
	return Error{
		Code:       common.PtrString("NoSuchUpload"),
		Message:    common.PtrString("The specified multipart upload does not exist. The upload ID might be invalid, or the multipart upload might have been aborted or completed."),
		BucketName: common.PtrString(bucketName),
		Resource:   common.PtrString(resPath),
		Key:        common.PtrString(key),
	}
}

// NotImplemented 501
func NotImplemented(bucketName, resPath string, key *string) Error {
	return Error{
		Code:       common.PtrString("NotImplemented"),
		Message:    common.PtrString("A header you provided implies functionality that is not implemented"),
		BucketName: common.PtrString(bucketName),
		Resource:   common.PtrString(resPath),
		Key:        key,
	}
}

// PreconditionFailed 412
func PreconditionFailed(bucketName, resPath, message string, key *string) Error {
	return Error{
		Code:       common.PtrString("PreconditionFailed"),
		Message:    common.PtrString(message),
		BucketName: common.PtrString(bucketName),
		Resource:   common.PtrString(resPath),
		Key:        key,
	}
}

// RequestIsNotMultiPartContent 400
func RequestIsNotMultiPartContent(bucketName, resPath string) Error {
	return Error{
		Code:       common.PtrString("RequestIsNotMultiPartContent"),
		Message:    common.PtrString("The Bucket POST request is not multi-part content."),
		BucketName: common.PtrString(bucketName),
		Resource:   common.PtrString(resPath),
	}
}

// SignatureDoesNotMatch 403
func SignatureDoesNotMatch(bucketName, resPath string, key *string) Error {
	return Error{
		Code:       common.PtrString("SignatureDoesNotMatch"),
		Message:    common.PtrString("The request signature we calculated does not match the signature you provided. Check your AWS Secret Access Key and signing method."),
		BucketName: common.PtrString(bucketName),
		Resource:   common.PtrString(resPath),
		Key:        key,
	}
}

// UnexpectedContent 400
