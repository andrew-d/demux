package main

import (
	"bytes"
)

type SshProtocol struct {
}

func init() {
	RegisterProtocol(SshProtocol{})
}

func (p SshProtocol) Detect(buf []byte) (bool, error) {
	if len(buf) < 4 {
		return false, ErrMoreData
	}

	return bytes.Equal(buf[0:4], []byte("SSH-")), nil
}

func (p SshProtocol) Name() string {
	return "ssh"
}
