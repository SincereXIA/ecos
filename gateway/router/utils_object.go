package router

import (
	"ecos/client"
	"ecos/client/io"
	"ecos/utils/errno"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/textproto"
	"regexp"
	"strconv"
	"strings"
)

/* Object Section of Route Utils */
func getFactory(c *gin.Context, bucketName, key string) (factory *io.EcosIOFactory, err error) {
	factory, err = Client.GetIOFactory(bucketName)
	if err != nil {
		if strings.Contains(err.Error(), errno.InfoNotFound.Error()) {
			c.XML(http.StatusNotFound, NoSuchBucket(bucketName))
			return
		}
		c.XML(http.StatusInternalServerError, InternalError(err.Error(), bucketName, c.Request.URL.Path, &key))
		return
	}
	return
}

func getObjectOperator(c *gin.Context, bucketName, key string) (operator *client.ObjectOperator, err error) {
	bucketRet, err := Client.GetVolumeOperator().Get(bucketName)
	if err != nil {
		if strings.Contains(err.Error(), errno.InfoNotFound.Error()) {
			c.XML(http.StatusNotFound, NoSuchBucket(bucketName))
			return
		}
		c.XML(http.StatusInternalServerError, InternalError(err.Error(), bucketName, c.Request.URL.Path, nil))
	}
	bucketOp := bucketRet.(*client.BucketOperator)
	objRet, err := bucketOp.Get(key)
	if err != nil {
		if strings.Contains(err.Error(), errno.InfoNotFound.Error()) {
			c.XML(http.StatusNotFound, NoSuchKey(bucketName, c.Request.URL.Path, key))
			return
		}
		c.XML(http.StatusInternalServerError, InternalError(err.Error(), bucketName, c.Request.URL.Path, &key))
		return
	}
	operator = objRet.(*client.ObjectOperator)
	return
}

// Codes Below from golang.org/x/net/http/fs.go
// License: github.com/golang/go/LICENSE

/*
Copyright (c) 2009 The Go Authors. All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are
met:

   * Redistributions of source code must retain the above copyright
notice, this list of conditions and the following disclaimer.
   * Redistributions in binary form must reproduce the above
copyright notice, this list of conditions and the following disclaimer
in the documentation and/or other materials provided with the
distribution.
   * Neither the name of Google Inc. nor the names of its
contributors may be used to endorse or promote products derived from
this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
"AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
*/

type httpRange struct {
	start, length int64
}

func (r httpRange) contentRange(size int64) string {
	return fmt.Sprintf("bytes %d-%d/%d", r.start, r.start+r.length-1, size)
}

// parseRange parses a Range header string as per RFC 7233.
// errNoOverlap is returned if none of the ranges overlap.
func parseRange(s string, size int64) ([]httpRange, error) {
	if s == "" {
		return nil, nil // header not present
	}
	const b = "bytes="
	if !strings.HasPrefix(s, b) {
		return nil, errors.New("invalid range")
	}
	var ranges []httpRange
	noOverlap := false
	for _, ra := range strings.Split(s[len(b):], ",") {
		ra = textproto.TrimString(ra)
		if ra == "" {
			continue
		}
		i := strings.Index(ra, "-")
		if i < 0 {
			return nil, errors.New("invalid range")
		}
		start, end := textproto.TrimString(ra[:i]), textproto.TrimString(ra[i+1:])
		var r httpRange
		if start == "" {
			// If no start is specified, end specifies the
			// range start relative to the end of the file.
			i, err := strconv.ParseInt(end, 10, 64)
			if err != nil {
				return nil, errors.New("invalid range")
			}
			if i > size {
				i = size
			}
			r.start = size - i
			r.length = size - r.start
		} else {
			i, err := strconv.ParseInt(start, 10, 64)
			if err != nil || i < 0 {
				return nil, errors.New("invalid range")
			}
			if i >= size {
				// If the range begins after the size of the content,
				// then it does not overlap.
				noOverlap = true
				continue
			}
			r.start = i
			if end == "" {
				// If no end is specified, range extends to end of the file.
				r.length = size - r.start
			} else {
				i, err := strconv.ParseInt(end, 10, 64)
				if err != nil || r.start > i {
					return nil, errors.New("invalid range")
				}
				if i >= size {
					i = size - 1
				}
				r.length = i - r.start + 1
			}
		}
		ranges = append(ranges, r)
	}
	if noOverlap && len(ranges) == 0 {
		// The specified ranges did not overlap with the content.
		return nil, errors.New("invalid range")
	}
	return ranges, nil
}

// Source codes from golang.org/x/net/http/fs.go ends here
// Codes Above from golang.org/x/net/http/fs.go

func parseSignature(line string) (int64, string, error) {
	r := regexp.MustCompile("([A-Fa-f\\d]+);chunk-signature=(\\w+)\r\n")
	res := r.FindStringSubmatch(line)
	if len(res) != 3 {
		return 0, "", errno.SignatureDoesNotMatch
	}
	length, err := strconv.ParseInt(res[1], 16, 64)
	if err != nil {
		return 0, "", err
	}
	return length, res[2], nil
}
