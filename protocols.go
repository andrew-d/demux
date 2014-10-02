package main

import (
	"errors"
	"fmt"
)

type Protocol interface {
	// Check if the given buffer matches this protocol.
	// Should return ErrMoreData if the buffer isn't large enough
	Detect(buf []byte) (bool, error)

	// Should return the name of the protocol.
	Name() string
}

var (
	ErrMoreData = errors.New("need more data")
	protocols   = make(map[string]Protocol)
)

func RegisterProtocol(p Protocol) {
	name := p.Name()

	if _, found := protocols[name]; found {
		panic(fmt.Sprintf("protocol '%s' already exists", name))
	}

	protocols[name] = p
}
