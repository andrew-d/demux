// +build linux

package main

import (
	"fmt"
	"net"
)

func openTransparent(addr string) (net.Conn, error) {
	// TODO: should open and return this
	return net.Dial("tcp", addr)
}
