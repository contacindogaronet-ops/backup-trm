package main

import (
	"errors"
	"net"
	"strconv"
)

var (
	errSocksVersion   = errors.New("unsupported socks version")
	errBlockedByRule  = errors.New("connection blocked by rule")
	errUnknownAtyp    = errors.New("unknown address type")
	errProxyTurnedOff = errors.New("proxy engine state is OFF")
)

func ReadFullNative(c net.Conn, buf []byte) error {
	total := 0
	target := len(buf)
	for total < target {
		n, err := c.Read(buf[total:])
		if err != nil {
			return err
		}
		total += n
	}
	return nil
}

type SOCKS5Request struct {
	Cmd      byte
	DestAddr string
	DestPort uint16
	Tier     byte
}

func HandshakeSOCKS5(conn net.Conn, rm *RuleManager, isEngineActive bool) (*SOCKS5Request, error) {
	if !isEngineActive {
		return nil, errProxyTurnedOff
	}

	var header [2]byte
	if err := ReadFullNative(conn, header[:]); err != nil {
		return nil, err
	}

	if header[0] != SocksVersion5 {
		return nil, errSocksVersion
	}

	numMethods := int(header[1])
	methodsBuf := make([]byte, numMethods)
	if err := ReadFullNative(conn, methodsBuf); err != nil {
		return nil, err
	}

	// SOCKS5 Response: 0x05, 0x00 (NO AUTH)
	if _, err := conn.Write([]byte{SocksVersion5, MethodNoAuth}); err != nil {
		return nil, err
	}

	var reqHeader [4]byte
	if err := ReadFullNative(conn, reqHeader[:]); err != nil {
		return nil, err
	}

	if reqHeader[0] != SocksVersion5 {
		return nil, errSocksVersion
	}

	cmd := reqHeader[1]
	atyp := reqHeader[3]

	var destAddr string
	var tier byte = CodeReguler

	switch atyp {
	case AtypIPv4:
		var ipBuf [4]byte
		if err := ReadFullNative(conn, ipBuf[:]); err != nil {
			return nil, err
		}
		ip := net.IP(ipBuf[:])
		tier = rm.EvaluateIP(ip)
		destAddr = ip.String()

	case AtypDomain:
		var lenBuf [1]byte
		if err := ReadFullNative(conn, lenBuf[:]); err != nil {
			return nil, err
		}
		dLen := int(lenBuf[0])
		dBuf := make([]byte, dLen)
		if err := ReadFullNative(conn, dBuf); err != nil {
			return nil, err
		}
		destAddr = string(dBuf)
		tier = rm.EvaluateDomain(destAddr)

	case AtypIPv6:
		var ip6Buf [16]byte
		if err := ReadFullNative(conn, ip6Buf[:]); err != nil {
			return nil, err
		}
		ip6 := net.IP(ip6Buf[:])
		tier = rm.EvaluateIP(ip6)
		destAddr = ip6.String()

	default:
		_, _ = conn.Write([]byte{SocksVersion5, RepAtypNotSupported, 0x00, AtypIPv4, 0, 0, 0, 0, 0, 0})
		return nil, errUnknownAtyp
	}

	var portBuf [2]byte
	if err := ReadFullNative(conn, portBuf[:]); err != nil {
		return nil, err
	}
	destPort := uint16(portBuf[0])<<8 | uint16(portBuf[1])

	req := &SOCKS5Request{
		Cmd:      cmd,
		DestAddr: destAddr,
		DestPort: destPort,
		Tier:     tier,
	}

	if tier == CodeBlock {
		_, _ = conn.Write([]byte{SocksVersion5, RepRuleFailure, 0x00, AtypIPv4, 0, 0, 0, 0, 0, 0})
		return req, errBlockedByRule
	}

	if cmd != CmdConnect && cmd != CmdUDPAssociate {
		_, _ = conn.Write([]byte{SocksVersion5, RepCmdNotSupported, 0x00, AtypIPv4, 0, 0, 0, 0, 0, 0})
		return req, errors.New("unsupported socks command")
	}

	return req, nil
}

// SendSocksSuccess mengembalikan BND.ADDR persis sesuai alamat socket lokal yang dikontak client
func SendSocksSuccess(clientConn net.Conn) error {
	localTCP, ok := clientConn.LocalAddr().(*net.TCPAddr)
	if !ok {
		// Fallback RFC default
		resp := [10]byte{SocksVersion5, RepSuccess, 0x00, AtypIPv4, 127, 0, 0, 3, 0x07, 0xD7}
		_, err := clientConn.Write(resp[:])
		return err
	}

	ip4 := localTCP.IP.To4()
	port := uint16(localTCP.Port)

	if ip4 != nil {
		resp := [10]byte{
			SocksVersion5,
			RepSuccess,
			0x00,
			AtypIPv4,
			ip4[0], ip4[1], ip4[2], ip4[3],
			byte(port >> 8),
			byte(port & 0xFF),
		}
		_, err := clientConn.Write(resp[:])
		return err
	}

	// IPv6 Handler
	ip6 := localTCP.IP.To16()
	resp := make([]byte, 0, 22)
	resp = append(resp, SocksVersion5, RepSuccess, 0x00, AtypIPv6)
	resp = append(resp, ip6...)
	resp = append(resp, byte(port>>8), byte(port&0xFF))
	_, err := clientConn.Write(resp)
	return err
}

func SendSocksUDPResponse(clientConn net.Conn, bindPort uint16) error {
	localTCP, ok := clientConn.LocalAddr().(*net.TCPAddr)
	ip4 := []byte{127, 0, 0, 3}
	if ok && localTCP.IP.To4() != nil {
		ip4 = localTCP.IP.To4()
	}

	resp := [10]byte{
		SocksVersion5,
		RepSuccess,
		0x00,
		AtypIPv4,
		ip4[0], ip4[1], ip4[2], ip4[3],
		byte(bindPort >> 8),
		byte(bindPort & 0xFF),
	}
	_, err := clientConn.Write(resp[:])
	return err
}

func FormatAddrPort(addr string, port uint16) string {
	buf := make([]byte, 0, len(addr)+6)
	buf = append(buf, addr...)
	buf = append(buf, ':')
	buf = strconv.AppendUint(buf, uint64(port), 10)
	return string(buf)
}