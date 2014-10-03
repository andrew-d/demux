// +build !linux

package main

import (
	"net"
)

func openTransparent(backendAddr, originalAddr *net.TCPAddr) (net.Conn, error) {
	// Not supported, so just open regularly
	return net.DialTCP("tcp", nil, backendAddr)
}
