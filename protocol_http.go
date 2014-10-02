package main

import (
	"bytes"
)

type HttpProtocol struct {
}

var httpMethods = []string{
	"GET",
	"PUT",
	"HEAD",
	"POST",
	"TRACE",
	"DELETE",
	"CONNECT",
	"OPTIONS",
}

func init() {
	RegisterProtocol(HttpProtocol{})
}

func (p HttpProtocol) Detect(buf []byte) (bool, error) {
	// If it's got "HTTP" in the request, it's HTTP
	if bytes.Contains(buf, []byte("HTTP")) {
		return true, nil
	}

	// Check each of the HTTP methods.  Note that we need to track the number of
	// methods where the buffer isn't long enough - we only return a definitive
	// 'false' if none of the methods result in ErrMoreData.
	var successful int
	for _, method := range httpMethods {
		if len(buf) < len(method) {
			// Not enough data
			continue
		}

		if bytes.Contains(buf, []byte(method)) {
			return true, nil
		}
	}

	// If we get here, then we're not successful.  Return 'ErrMoreData' if not all
	// the above checks succeeded.
	if successful != len(httpMethods) {
		return false, ErrMoreData
	}

	return false, nil
}

func (p HttpProtocol) Name() string {
	return "http"
}
