// +build linux

package main

import (
	"net"
	"os"
	"syscall"
)

/*

Steps to get transparent proxying working on Linux:

1) Run these commands (as root):

	iptables -t mangle -N DEMUX
	iptables -t mangle -A DEMUX --jump MARK --set-mark 0x1
	iptables -t mangle -A DEMUX --jump ACCEPT
	ip rule add fwmark 0x1 lookup 100
	ip route add local 0.0.0.0/0 dev lo table 100

2) Run the following commands - note that there's one for each interface/port
   that you forward to:

	iptables -t mangle -A OUTPUT --protocol tcp --out-interface eth0 --sport 22 --jump DEMUX
	iptables -t mangle -A OUTPUT --protocol tcp --out-interface eth0 --sport 8080 --jump DEMUX

3) Finally, run demux (needs to be as root to use transparent proxying):

	sudo ./demux -p 5555 --transparent=true \
				 --http-destination=<eth0 address>:8080 \
				 --ssh-destination=<eth0 address>:22

   Note that the various destination addresses must be specified with the same
   address as the interface you gave in step 2.

*/

func openTransparent(backendAddr, originalAddr *net.TCPAddr) (net.Conn, error) {
	family := tcpAddrFamily(backendAddr)
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

	// Set IP_TRANSPARENT socket option
	err = os.NewSyscallError("setsockopt",
		syscall.SetsockoptInt(s, syscall.IPPROTO_IP, syscall.IP_TRANSPARENT, 1))
	if err != nil {
		return nil, err
	}

	family = tcpAddrFamily(originalAddr)
	laddr, err := ipToSockaddr(family, originalAddr.IP, originalAddr.Port, originalAddr.Zone)
	if err != nil {
		return nil, err
	}

	if err := syscall.Bind(s, laddr); err != nil {
		return nil, err
	}

	family = tcpAddrFamily(backendAddr)
	raddr, err := ipToSockaddr(family, backendAddr.IP, backendAddr.Port, backendAddr.Zone)
	if err != nil {
		return nil, err
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

// NOTE: Taken from the Go source: src/net/tcpsock_posix.go
func tcpAddrFamily(a *net.TCPAddr) int {
	if a == nil || len(a.IP) <= net.IPv4len {
		return syscall.AF_INET
	}
	if a.IP.To4() != nil {
		return syscall.AF_INET
	}
	return syscall.AF_INET6
}

// NOTE: Taken from the Go source: src/net/ipsock_posix.go
func ipToSockaddr(family int, ip net.IP, port int, zone string) (syscall.Sockaddr, error) {
	switch family {
	case syscall.AF_INET:
		if len(ip) == 0 {
			ip = net.IPv4zero
		}
		if ip = ip.To4(); ip == nil {
			return nil, net.InvalidAddrError("non-IPv4 address")
		}
		sa := new(syscall.SockaddrInet4)
		for i := 0; i < net.IPv4len; i++ {
			sa.Addr[i] = ip[i]
		}
		sa.Port = port
		return sa, nil
	case syscall.AF_INET6:
		if len(ip) == 0 {
			ip = net.IPv6zero
		}
		// IPv4 callers use 0.0.0.0 to mean "announce on any available address".
		// In IPv6 mode, Linux treats that as meaning "announce on 0.0.0.0",
		// which it refuses to do.  Rewrite to the IPv6 unspecified address.
		if ip.Equal(net.IPv4zero) {
			ip = net.IPv6zero
		}
		if ip = ip.To16(); ip == nil {
			return nil, net.InvalidAddrError("non-IPv6 address")
		}
		sa := new(syscall.SockaddrInet6)
		for i := 0; i < net.IPv6len; i++ {
			sa.Addr[i] = ip[i]
		}
		sa.Port = port
		sa.ZoneId = uint32(zoneToInt(zone))
		return sa, nil
	}
	return nil, net.InvalidAddrError("unexpected socket family")
}

func zoneToInt(zone string) int {
	if zone == "" {
		return 0
	}
	if ifi, err := net.InterfaceByName(zone); err == nil {
		return ifi.Index
	}
	n, _, _ := dtoi(zone, 0)
	return n
}

// Bigger than we need, not too big to worry about overflow
const big = 0xFFFFFF

// Decimal to integer starting at &s[i0].
// Returns number, new offset, success.
func dtoi(s string, i0 int) (n int, i int, ok bool) {
	n = 0
	for i = i0; i < len(s) && '0' <= s[i] && s[i] <= '9'; i++ {
		n = n*10 + int(s[i]-'0')
		if n >= big {
			return 0, i, false
		}
	}
	if i == i0 {
		return 0, i, false
	}
	return n, i, true
}
