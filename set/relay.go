package main

import (
	"net"
	"time"

	"github.com/rs/zerolog/log"
	"golang.org/x/sys/unix"
)

// ==========================================
// ROUTING, SPLICE TIERING & ORCHESTRATOR
// ==========================================
func executeRelay(client *net.TCPConn, target string, targetKey []byte, targetIP net.IP) {
	tier := CodeReguler
	matched := false

	// Cek Rule berbasis IP/CIDR (Jika format target adalah murni IP)
	if targetIP != nil {
		for i := 0; i < len(IPRules); i++ {
			if IPRules[i].Net.Contains(targetIP) {
				tier = IPRules[i].Tier
				matched = true
				break
			}
		}
	}

	// Fallback: Jika tidak tembus CIDR (Domain Murni), cari di FastCache + Subdomain Scan
	if !matched {
		var valBuf []byte
		
		// 1. Cek Exact Match
		valBuf = DBEngine.Get(valBuf[:0], targetKey)
		if len(valBuf) > 0 {
			tier = valBuf[0]
		} else {
			// 2. Zero-Alloc Subdomain Scanner
			for i := 0; i < len(targetKey); i++ {
				if targetKey[i] == '.' && i+1 < len(targetKey) {
					valBuf = DBEngine.Get(valBuf[:0], targetKey[i+1:])
					if len(valBuf) > 0 {
						tier = valBuf[0]
						break
					}
				}
			}
		}
	}

	if tier == CodeBlock {
		TotalBlocked.Add(1)
		log.Error().Str("target", target).Msg("BLOCKED BY DB/CIDR")
		client.Write([]byte{0x05, 0x02, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		client.Close()
		return
	}

	conn, err := net.DialTimeout("tcp", target, 5*time.Second)
	if err != nil {
		client.Write([]byte{0x05, 0x05, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		client.Close()
		return
	}
	remote := conn.(*net.TCPConn)
	tuneSocket(remote)

	client.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})

	TotalAllowed.Add(1)
	ActiveConns.Add(1)
	log.Info().Str("target", target).Uint8("tier", tier).Msg("TERHUBUNG")

	bufSize := BaseBuffer
	switch tier {
	case CodeVIP:
		bufSize = BaseBuffer / 2
	case CodeReguler:
		bufSize = BaseBuffer / 6
	}

	go func() {
		defer client.Close()
		defer remote.Close()
		defer ActiveConns.Add(-1)

		errChan := make(chan struct{}, 2)
		go func() {
			syncData(client, remote, bufSize)
			errChan <- struct{}{}
		}()
		go func() {
			syncData(remote, client, bufSize)
			errChan <- struct{}{}
		}()

		<-errChan
	}()
}

func syncData(dst *net.TCPConn, src *net.TCPConn, bufSize int) {
	sysDst, errDst := dst.SyscallConn()
	sysSrc, errSrc := src.SyscallConn()
	if errDst != nil || errSrc != nil {
		return
	}

	var pipeFDs [2]int
	if err := unix.Pipe2(pipeFDs[:], unix.O_NONBLOCK); err != nil {
		return
	}
	unix.FcntlInt(uintptr(pipeFDs[0]), unix.F_SETPIPE_SZ, bufSize)

	defer unix.Close(pipeFDs[0])
	defer unix.Close(pipeFDs[1])

	for {
		var nSplice int
		var errSplice error

		errRead := sysSrc.Read(func(srcFD uintptr) bool {
			n, err := unix.Splice(int(srcFD), nil, pipeFDs[1], nil, bufSize, unix.SPLICE_F_MOVE|unix.SPLICE_F_NONBLOCK)
			if err == unix.EAGAIN || err == unix.EWOULDBLOCK {
				return false
			}
			nSplice = int(n)
			errSplice = err
			return true
		})

		if errRead != nil || errSplice != nil || nSplice == 0 {
			break
		}

		written := 0
		for written < nSplice {
			var nOut int
			errWrite := sysDst.Write(func(dstFD uintptr) bool {
				n, err := unix.Splice(pipeFDs[0], nil, int(dstFD), nil, nSplice-written, unix.SPLICE_F_MOVE|unix.SPLICE_F_NONBLOCK)
				if err == unix.EAGAIN || err == unix.EWOULDBLOCK {
					return false
				}
				nOut = int(n)
				errSplice = err
				return true
			})

			if errWrite != nil || errSplice != nil || nOut == 0 {
				break
			}
			written += nOut
		}

		if written < nSplice {
			break
		}
	}
}
