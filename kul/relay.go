package main

import (
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// ==========================================
// DYNAMIC TIER-BASED MEMORY POOL
// ==========================================
var RegulerPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 32*1024)
		return &b
	},
}

var VVIPPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 4*1024*1024)
		return &b
	},
}

var StandardDialer = &net.Dialer{
	Timeout:   5 * time.Second,
	KeepAlive: 30 * time.Second,
}

// ==========================================
// 🧠 HELPER: LOGGING FILTER
// ==========================================
func isNormalClose(err error) bool {
	if err == nil { return false }
	// Filter otomatis: abaikan noise "closed network connection"
	return strings.Contains(err.Error(), "use of closed network connection")
}

// ==========================================
// KERNEL-FRIENDLY TUNE
// ==========================================
func tuneSocket(conn *net.TCPConn) {
	conn.SetNoDelay(true)
	conn.SetKeepAlive(true)
}

// ==========================================
// THE SPLICER (ZERO-COPY)
// ==========================================
func zeroCopy(dst net.Conn, src net.Conn, tier byte) {
	var bufPtr *[]byte
	
	if tier == CodeVVIP {
		bufPtr = VVIPPool.Get().(*[]byte)
		defer VVIPPool.Put(bufPtr)
	} else {
		bufPtr = RegulerPool.Get().(*[]byte)
		defer RegulerPool.Put(bufPtr)
	}

	_, err := io.CopyBuffer(dst, src, *bufPtr)
	if err != nil && !isNormalClose(err) && DebugMode {
		log.Debug().Err(err).Msg("Splicer: Copy error")
	}
}

// ==========================================
// 🔴 EXECUTE RELAY (THE CLEAN STANDARD)
// ==========================================
func executeRelay(client *net.TCPConn, target string, successReply []byte, tier byte) {
	remote, err := StandardDialer.Dial("tcp", target)
	if err != nil {
		if DebugMode {
			log.Debug().Str("target", target).Err(err).Msg("Relay: Dial failed")
		}
		client.Close()
		return
	}

	if successReply != nil {
		client.Write(successReply)
	}

	remoteTCP, ok := remote.(*net.TCPConn)
	if ok {
		tuneSocket(remoteTCP)
	}
	tuneSocket(client)

	ActiveConns.Add(1)

	relayTask := func() {
		defer ActiveConns.Add(-1)
		defer client.Close()
		defer remote.Close()
		
		if DebugMode {
			log.Debug().Str("target", target).Msg("Relay: Connection established")
		}

		done := make(chan struct{}, 2)
		
		go func() {
			zeroCopy(client, remote, tier)
			done <- struct{}{} 
		}()
		
		go func() {
			zeroCopy(remote, client, tier)
			done <- struct{}{} 
		}()
		
		<-done
		
		if DebugMode {
			log.Debug().Str("target", target).Msg("Relay: Connection closed naturally")
		}
	}

	if err := GPool.Submit(relayTask); err != nil {
		go relayTask() 
	}
}
