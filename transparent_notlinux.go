// +build !linux

package main

import (
	"net"
)

func openTransparent(addr string) (net.Conn, error) {
	// Not supported, so just open regularly
	return net.Dial("tcp", addr)
}
