package iouringnet

import (
	"encoding/binary"
	"fmt"
	"github.com/iceber/iouring-go"
	"log"
	"net"
	"syscall"
)

type Listener struct {
	fd              int
	addr            string
	iour            *iouring.IOURing
	resulter        chan iouring.Result
	newConnectorFun func(fd int, addr *syscall.SockaddrInet4) UserConnector
}

func NewListener(addr string, newConnectorFun func(fd int, addr *syscall.SockaddrInet4) UserConnector) (*Listener, error) {
	fd, err := makeFd(addr)
	if err != nil {
		return nil, err
	}

	iour, err := iouring.New(1024)
	if err != nil {
		return nil, fmt.Errorf("new iouring error:%v", err)
	}

	resulter := make(chan iouring.Result, 10)

	listener := &Listener{
		fd:              fd,
		addr:            addr,
		iour:            iour,
		resulter:        resulter,
		newConnectorFun: newConnectorFun,
	}

	return listener, nil
}

func (l *Listener) Start() error {
	err := l.startAccept()
	if err != nil {
		return err
	}

	for {
		select {
		case result, ok := <-l.resulter:
			if !ok {
				return nil
			}
			switch result.Opcode() {
			case iouring.OpAccept:
				l.startAccept()
				l.handleNewConnector(result)
			case iouring.OpRead:
				l.handleRead(result)
			case iouring.OpWrite:
				// write successful
			case iouring.OpClose:
				c, ok := result.GetRequestInfo().(*Connector)
				if ok {
					log.Printf("[server] client[%v] close with reason:%v\n", c.fd, result.Err())
					c.stop()
				} else {
					log.Printf("[server] client[%v] close with reason:%v\n", c.fd, result.Err())
				}
			}
		}
	}

	return nil
}

func (l *Listener) handleNewConnector(result iouring.Result) error {
	if err := result.Err(); err != nil {
		return fmt.Errorf("accept error: %v", err)
	}

	connFd := result.ReturnValue0().(int)
	sockaddr := result.ReturnValue1().(*syscall.SockaddrInet4)

	clientAddr := fmt.Sprintf("%s:%d", net.IPv4(sockaddr.Addr[0], sockaddr.Addr[1], sockaddr.Addr[2], sockaddr.Addr[3]), sockaddr.Port)
	log.Printf("[server] client[%v] connect with addr:%v\n", connFd, clientAddr)

	//fmt.Printf("Client Conn: %s\n", clientAddr)

	userConnector := l.newConnectorFun(connFd, sockaddr)

	connector := newConnector(connFd, sockaddr, userConnector)
	go connector.start()

	// 读8个字节头，4字节tag,4字节length
	headerBuffer := make([]byte, 8)
	prep := iouring.Read(connFd, headerBuffer).WithInfo(connector)
	if _, err := l.iour.SubmitRequest(prep, l.resulter); err != nil {
		return fmt.Errorf("submit read request error: %v", err)
	}

	return nil
}

func (l *Listener) closeConnector(c *Connector) {
	prep := iouring.Close(c.fd).WithInfo(c)
	if _, err := l.iour.SubmitRequest(prep, l.resulter); err != nil {
		panic(fmt.Errorf("[%s] submit write request error: %v", c.fd, err))
	}
}

func (l *Listener) handleRead(result iouring.Result) error {
	c := result.GetRequestInfo().(*Connector)
	if err := result.Err(); err != nil {
		return fmt.Errorf("[%s] read error: %v", c.fd, err)
	}

	num := result.ReturnValue0().(int)
	buf, _ := result.GetRequestBuffer()

	if c.incomeHeaderTag == 0 && num == 8 {
		header := buf[:num]
		tag := binary.BigEndian.Uint32(header)
		length := binary.BigEndian.Uint32(header[4:])

		//log.Printf("receive from client[%v] header msg:%v", c.fd, tag)

		if length >= 2<<20 {
			log.Printf("client[%v] receive invalid packet with length:%v, close it", c.fd, length)
			// 设定一个包最大长度，超过认为非法，就关闭套接字
			l.closeConnector(c)
			return nil
		}
		c.incomeHeaderTag = tag

		contentBuffer := make([]byte, length)
		prep := iouring.Read(c.fd, contentBuffer).WithInfo(c)
		if _, err := l.iour.SubmitRequest(prep, l.resulter); err != nil {
			return fmt.Errorf("submit read request error: %v", err)
		}
	} else if c.incomeHeaderTag != 0 {
		content := buf[:num]
		//log.Printf("receive from client[%v] content msg:%v", c.fd, string(content))
		c.handleRead(&TlvPacket{Tag: c.incomeHeaderTag, Payload: content})

		// 继续读下个包
		// 读8个字节头，4字节tag,4字节length
		c.incomeHeaderTag = 0
		headerBuffer := make([]byte, 8)
		prep := iouring.Read(c.fd, headerBuffer).WithInfo(c)
		if _, err := l.iour.SubmitRequest(prep, l.resulter); err != nil {
			return fmt.Errorf("submit read request error: %v", err)
		}
	}

	return nil
}

func (l *Listener) startAccept() error {
	if _, err := l.iour.SubmitRequest(iouring.Accept(l.fd), l.resulter); err != nil {
		return fmt.Errorf("submit accept request error: %v", err)
	}

	log.Printf("[server] server start listen on:%v\n", l.addr)

	return nil
}

func makeFd(addr string) (int, error) {
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	if err != nil {
		return -1, fmt.Errorf("Socket error:%v", err)
	}

	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return -1, fmt.Errorf("ResolveTCPAddr error:%v", err)
	}

	sockaddr := &syscall.SockaddrInet4{Port: tcpAddr.Port}
	copy(sockaddr.Addr[:], tcpAddr.IP.To4())
	if err := syscall.Bind(fd, sockaddr); err != nil {
		return -1, fmt.Errorf("Bind error:%v", err)
	}

	if err := syscall.Listen(fd, syscall.SOMAXCONN); err != nil {
		return -1, fmt.Errorf("Listen error:%v", err)
	}
	return fd, nil
}
