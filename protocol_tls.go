package main

type TlsProtocol struct {
}

func init() {
	RegisterProtocol(TlsProtocol{})
}

func (p TlsProtocol) Detect(buf []byte) (bool, error) {
	if len(buf) < 3 {
		return false, ErrMoreData
	}

	// TLS packets start with a "Hello" record (type 0x16), followed by the
	// version.
	// TODO: This currently doesn't support SSLv2!
	return (buf[0] == 0x16 && buf[1] == 0x03 && (buf[2] >= 0x00 && buf[2] <= 0x03)), nil
}

func (p TlsProtocol) Name() string {
	return "tls"
}
