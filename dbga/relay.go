package main

import (
	"net"
	"sync"
	"time"
)

var RelayPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 32*1024)
		return &b
	},
}

func tuneSocket(conn *net.TCPConn) {
	conn.SetNoDelay(true)
	conn.SetReadBuffer(BaseBuffer)
	conn.SetWriteBuffer(BaseBuffer)
}

func executeRelay(client *net.TCPConn, target string, targetKey []byte, targetIP net.IP) {
	remote, err := net.DialTimeout("tcp", target, 5*time.Second)
	if err != nil {
		client.Close()
		return
	}

	remoteTCP, ok := remote.(*net.TCPConn)
	if ok {
		tuneSocket(remoteTCP)
	}

	ActiveConns.Add(1)
	TotalAllowed.Add(1)

	_ = GPool.Submit(func() {
		defer client.Close()
		defer remote.Close()
		defer ActiveConns.Add(-1)

		done := make(chan struct{}, 2)

		go func() {
			zeroCopy(remote, client)
			done <- struct{}{}
		}()

		zeroCopy(client, remote)
		<-done
	})
}

// 🔴 LOW-LEVEL RELAY ENGINE (PENGGANTI io.Copy)
func zeroCopy(dst net.Conn, src net.Conn) {
	bufPtr := RelayPool.Get().(*[]byte)
	defer RelayPool.Put(bufPtr)
	buf := *bufPtr

	for {
		n, err := src.Read(buf)
		if n > 0 {
			_, ew := dst.Write(buf[:n])
			if ew != nil {
				break
			}
		}
		if err != nil {
			break
		}
	}
}
