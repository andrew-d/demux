package main

import (
	"encoding/binary"
)

type OpenVpnProtocol struct {
}

func init() {
	RegisterProtocol(OpenVpnProtocol{})
}

func (p OpenVpnProtocol) Detect(buf []byte) (bool, error) {
	if len(buf) < 2 {
		return false, ErrMoreData
	}

	// TODO: this is rather fragile - it relies on us receiving the entirety of
	// the first packet immediately
	plen := int(binary.BigEndian.Uint16(buf[0:2]))
	return plen == len(buf)-2, nil
}

func (p OpenVpnProtocol) Name() string {
	return "openvpn"
}
