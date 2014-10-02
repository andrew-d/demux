// +build linux

package main

import (
	"net"
	"os"
	"syscall"
)

func openTransparent(addr string) (net.Conn, error) {
	family := syscall.AF_INET
	sotype := syscall.SOCK_STREAM
	proto := syscall.IPPROTO_TCP
	ipv6only := false

	// TODO: duplicate retry logic from standard library

	s, err := sysSocket(family, sotype, proto)
	if err != nil {
		return nil, err
	}

	// These are the default socket options for Linux from the standard library:
	// src/net/sockopt_linux.go
	if family == syscall.AF_INET6 && sotype != syscall.SOCK_RAW {
		// Allow both IP versions even if the OS default
		// is otherwise.  Note that some operating systems
		// never admit this option.
		syscall.SetsockoptInt(s, syscall.IPPROTO_IPV6, syscall.IPV6_V6ONLY, boolint(ipv6only))
	}

	// Allow broadcast.
	err = os.NewSyscallError("setsockopt",
		syscall.SetsockoptInt(s, syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1))
	if err != nil {
		return nil, err
	}

	// TODO: set IP_TRANSPARENT socket option

	// TODO: make this work
	// TODO: support IPv4 and IPv6
	laddr := &syscall.SockaddrInet4{
		Port: 0,
		Addr: [4]byte{0, 0, 0, 0},
	}

	if err := syscall.Bind(s, laddr); err != nil {
		return nil, err
	}

	// TODO: make this work
	raddr := &syscall.SockaddrInet4{
		Port: 0,
		Addr: [4]byte{0, 0, 0, 0},
	}
	if err := syscall.Connect(s, raddr); err != nil {
		return nil, err
	}

	// Convert socket to *os.File
	// Note that net.FileConn() returns a copy of the socket, so we close this
	// File on return
	f := os.NewFile(uintptr(s), "dial")
	defer f.Close()

	// Make a net.Conn from our file
	c, err := net.FileConn(f)
	if err != nil {
		return nil, err
	}

	return c, nil
}

// NOTE: Taken from the Go source: src/net/sock_cloexec.go
// Wrapper around the socket system call that marks the returned file
// descriptor as nonblocking and close-on-exec.
func sysSocket(family, sotype, proto int) (int, error) {
	s, err := syscall.Socket(family, sotype|syscall.SOCK_NONBLOCK|syscall.SOCK_CLOEXEC, proto)
	// On Linux the SOCK_NONBLOCK and SOCK_CLOEXEC flags were
	// introduced in 2.6.27 kernel and on FreeBSD both flags were
	// introduced in 10 kernel. If we get an EINVAL error on Linux
	// or EPROTONOSUPPORT error on FreeBSD, fall back to using
	// socket without them.
	if err == nil || (err != syscall.EPROTONOSUPPORT && err != syscall.EINVAL) {
		return s, err
	}

	// See ../syscall/exec_unix.go for description of ForkLock.
	syscall.ForkLock.RLock()
	s, err = syscall.Socket(family, sotype, proto)
	if err == nil {
		syscall.CloseOnExec(s)
	}
	syscall.ForkLock.RUnlock()
	if err != nil {
		return -1, err
	}
	if err = syscall.SetNonblock(s, true); err != nil {
		syscall.Close(s)
		return -1, err
	}
	return s, nil
}

// NOTE: Taken from the Go source: src/net/sockopt_posix.go
// Boolean to int.
func boolint(b bool) int {
	if b {
		return 1
	}
	return 0
}
