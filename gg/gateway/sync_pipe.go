package gateway

import (
	"io"
	"net"
	"sync"
	"sync/atomic" // 🚀 Pake Atomic, gak perlu nunggu lock!
	"nganjuk-engine-reborn/config"
)

var bufferPool = sync.Pool{ New: func() interface{} { b := make([]byte, 512*1024); return &b } }
var chunkedPool = sync.Pool{ New: func() interface{} { b := make([]byte, 32*1024); return &b } }

func SyncPipe(client, server net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	targetName := server.RemoteAddr().String()

	go func() {
		defer wg.Done()
		buf := bufferPool.Get().(*[]byte)
		defer bufferPool.Put(buf)
		n, _ := io.CopyBuffer(server, client, *buf)
		if n > 0 {
			// ⚡ ATOMIC: Gak pake Lock, langsung nambah! Melesat!
			atomic.AddInt64(&config.TotalBytes, n)
			config.BytesMutex.Lock()
			config.HostBytes[targetName] += n
			config.BytesMutex.Unlock()
		}
		server.Close()
	}()

	go func() {
		defer wg.Done()
		buf := bufferPool.Get().(*[]byte)
		defer bufferPool.Put(buf)
		n, _ := io.CopyBuffer(client, server, *buf)
		if n > 0 {
			atomic.AddInt64(&config.TotalBytes, n)
			config.BytesMutex.Lock()
			config.HostBytes[targetName] += n
			config.BytesMutex.Unlock()
		}
		client.Close()
	}()
	wg.Wait()
}

func SyncChunkedPipe(client, server net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	targetName := server.RemoteAddr().String()

	go func() {
		defer wg.Done()
		buf := chunkedPool.Get().(*[]byte)
		defer chunkedPool.Put(buf)
		n, _ := io.CopyBuffer(server, client, *buf)
		if n > 0 {
			atomic.AddInt64(&config.TotalBytes, n)
			config.BytesMutex.Lock()
			config.HostBytes[targetName] += n
			config.BytesMutex.Unlock()
		}
		server.Close()
	}()

	go func() {
		defer wg.Done()
		buf := chunkedPool.Get().(*[]byte)
		defer bufferPool.Put(buf)
		n, _ := io.CopyBuffer(client, server, *buf)
		if n > 0 {
			atomic.AddInt64(&config.TotalBytes, n)
			config.BytesMutex.Lock()
			config.HostBytes[targetName] += n
			config.BytesMutex.Unlock()
		}
		client.Close()
	}()
	wg.Wait()
}
