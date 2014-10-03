package main

import (
	"errors"
	"io"
	"net"
	"sync/atomic"

	log "gopkg.in/inconshreveable/log15.v2"
)

type Proxy struct {
	localConn  net.Conn
	remoteConn net.Conn
	logger     log.Logger
	errored    uint32
	errChan    chan struct{}

	// Config
	EnabledProtocols  []Protocol
	ProtoDestinations map[string]*net.TCPAddr
}

var (
	ErrNoProtos = errors.New("no protocols detected")
)

func NewProxy(c net.Conn, root log.Logger) *Proxy {
	logger := root.New(
		"remoteAddr", c.RemoteAddr(),
		"localAddr", c.LocalAddr(),
	)

	ret := &Proxy{
		remoteConn: c,
		logger:     logger,
		errChan:    make(chan struct{}),
	}
	return ret
}

func (p *Proxy) Start() {
	defer p.remoteConn.Close()

	p.logger.Info("Accepted")

	// Start by detecting the protocol in use.
	initial, name, err := p.detectProtocol()
	if err != nil {
		if err == ErrNoProtos {
			// TODO: handle this
			return
		} else {
			p.logger.Error("Error in initial read", "err", err)
			return
		}
	}

	p.logger.Debug("Protocol detected", "name", name)
	dest := p.ProtoDestinations[name]

	// Dial the backend.
	if flagUseTransparent {
		remoteAddr := p.remoteConn.RemoteAddr().(*net.TCPAddr)
		p.localConn, err = openTransparent(dest, remoteAddr)
	} else {
		p.localConn, err = net.DialTCP("tcp", nil, dest)
	}
	if err != nil {
		p.logger.Error("Error dialing backend", "err", err)
		return
	}
	defer p.localConn.Close()

	// Copy our initial buffer to the backend.
	// TODO: handle not-full write
	_, err = p.localConn.Write(initial)
	if err != nil {
		p.logger.Error("Error writing initial buff to backend", "err", err)
		return
	}

	p.logger.Debug("Initial write completed, started copying data...")

	// Start copying between the two connections
	go p.pipe(p.localConn, p.remoteConn)
	go p.pipe(p.remoteConn, p.localConn)

	// Wait for the error signal
	<-p.errChan

	p.logger.Info("Connection closed")
}

func (p *Proxy) detectProtocol() ([]byte, string, error) {
	var n, recvd int
	var valid bool
	var err error
	var proto Protocol

	buf := make([]byte, 512)

detect:
	for {
		n, err = p.remoteConn.Read(buf[recvd:])
		if err != nil {
			return nil, "", err
		}

		recvd += n
		p.logger.Debug("Did new read",
			"readSize", n,
			"totalRead", recvd,
		)

		// Try detecting.
		var detected int
		for _, proto = range p.EnabledProtocols {
			if valid, err = proto.Detect(buf[0:recvd]); valid {
				break detect
			}

			// Track how many protocols successfully detected, regardless of what the
			// result might be.
			if err == nil {
				detected += 1
				continue
			}

			if err != ErrMoreData {
				p.logger.Error("Got error while detecting",
					"err", err,
					"proto", proto.Name(),
				)
			}

			// err == ErrMoreData, ignore it
		}

		// If all the protocols failed to detect...
		if detected == len(p.EnabledProtocols) {
			p.logger.Warn("All protocols failed to detect")
			return buf[0:recvd], "", ErrNoProtos
		}
	}

	return buf[0:recvd], proto.Name(), nil
}

func (p *Proxy) pipe(src, dest net.Conn) {
	buf := make([]byte, 0xFFFF)
	for {
		n, err := src.Read(buf)
		if err != nil {
			p.markError("Error reading", err)
			return
		}

		// Write to destination
		_, err = dest.Write(buf[0:n])
		if err != nil {
			p.markError("Error writing", err)
			return
		}
	}
}

func (p *Proxy) markError(msg string, err error) {
	if !atomic.CompareAndSwapUint32(&p.errored, 0, 1) {
		// We didn't swap, which means that the value must be a value other than 0,
		// and thus we aren't the first to execute this.  Just exit.
		return
	}

	if err != io.EOF {
		p.logger.Error(msg, "err", err)
	}

	p.errChan <- struct{}{}
}
