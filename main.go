package main

import (
	"flag"
	"github.com/oxtoacart/bpool"
	"io"
	"log"
	"net"
	"strconv"
)

type Proxy struct {
	bufPool    *bpool.BytePool
	addr       string
	port       string
	auth       string
	credential string
	debug      bool
}

func main() {
	addr := flag.String("addr", "127.0.0.1", "监听地址， 默认 127.0.0.1")
	port := flag.String("port", "8999", "监听端口， 默认 8999")
	debug := flag.Bool("debug", true, "开启调试模式")
	flag.Parse()

	// 初始化
	proxy := &Proxy{
		bufPool: bpool.NewBytePool(5*1024*1024, 32*1024),
		addr:    *addr,
		port:    *port,
		debug:   *debug,
	}

	// 监听端口
	proxy.Debug("socks proxy running in %s:%s", *addr, *port)
	l, err := net.Listen("tcp", *addr + ":" + *port)
	if err != nil {
		log.Panic(err)
	}

	defer l.Close()
	for {
		conn, err := l.Accept()
		if err != nil {
			if err != nil {
				panic(err)
			}
			return
		}

		go handleConn(conn, proxy)
	}
}

func handleConn(client net.Conn, proxy *Proxy) {
	if client == nil {
		return
	}

	defer client.Close()

	var b[1024]byte
	n, err := client.Read(b[:])
	if err != nil {
		log.Println(err)
		return
	}

	if b[0] != 0x5 {
		proxy.Printf("not sockets 5: %v\n", b[0])
		return
	}

	//客户端回应：Socket服务端不需要验证方式
	client.Write([]byte{0x05, 0x00})
	n, err = client.Read(b[:])
	var host, port string
	switch b[3] {
	case 0x01: //IP V4
		host = net.IPv4(b[4], b[5], b[6], b[7]).String()
	case 0x03: //域名
		host = string(b[5 : n-2]) //b[4]表示域名的长度
	case 0x04: //IP V6
		host = net.IP{b[4], b[5], b[6], b[7], b[8], b[9], b[10], b[11], b[12], b[13], b[14], b[15], b[16], b[17], b[18], b[19]}.String()
	}
	port = strconv.Itoa(int(b[n-2])<<8 | int(b[n-1]))

	server, err := net.Dial("tcp", net.JoinHostPort(host, port))
	if err != nil {
		log.Println(err)
		return
	}
	defer server.Close()
	client.Write([]byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}) //响应客户端连接成功

	// 双向数据转发
	go func(src io.Writer, dst io.Reader) {
		n, err := proxy.Copy(src, dst)
		proxy.Debug("forward length %d from server to client, err: %v", n, err)
	}(server, client)

	n64, err := proxy.Copy(client, server)
	proxy.Debug("forward length %d from client to server, err: %v", n64, err)
}

// buffer 池复制
func (p *Proxy) Copy(dst io.Writer, src io.Reader) (written int64, err error) {
	buf := p.bufPool.Get()
	defer p.bufPool.Put(buf)
	return io.CopyBuffer(dst, src, buf)
}

// log.Printf
func (p *Proxy) Printf(format string, v ...interface{}) {
	log.Printf(format, v...)
}

// log.Printf 判断 debug 参数
func (p *Proxy) Debug(format string, v ...interface{}) {
	if !p.debug {
		return
	}
	log.Printf(format, v...)
}
