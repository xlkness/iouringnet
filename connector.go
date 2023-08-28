package iouringnet

import "syscall"

type UserConnector interface {
	// ......
	String() string
	PreHandleRequest(request *TlvPacket) (response *TlvPacket, userData interface{}, err error)
	HandleRequest(request *TlvPacket, userData interface{}) (response *TlvPacket, newUserData interface{}, err error)
	// ......
}

type Connector struct {
	fd              int
	addr            *syscall.SockaddrInet4
	readc           chan *TlvPacket
	writec          chan *TlvPacket
	incomeHeaderTag uint32
	uc              UserConnector
}

func newConnector(fd int, addr *syscall.SockaddrInet4, uc UserConnector) *Connector {
	c := &Connector{
		fd:     fd,
		addr:   addr,
		uc:     uc,
		readc:  make(chan *TlvPacket, 10),
		writec: make(chan *TlvPacket, 10),
	}
	return c
}

func (c *Connector) start() {
	for {
		select {
		case packet, ok := <-c.readc:
			if !ok {
				return
			}

			preResp, data, err := c.uc.PreHandleRequest(packet)
			if preResp != nil && preResp.Tag != 0 {
				syscall.Write(c.fd, packetTlv(preResp))
				continue
			}
			if err != nil {
				continue
			}
			resp, _, err := c.uc.HandleRequest(packet, data)
			if resp.Tag != 0 {
				syscall.Write(c.fd, packetTlv(resp))
			}
		case packet, ok := <-c.writec:
			if !ok {
				return
			}
			syscall.Write(c.fd, packetTlv(packet))
		}
	}
}

func (c *Connector) stop() {
	close(c.readc)
	close(c.writec)
}

func (c *Connector) handleRead(packet *TlvPacket) {
	select {
	case c.readc <- packet:
	default:
		// todo: log full
	}
}

func (c *Connector) write(packet *TlvPacket) {
	select {
	case c.writec <- packet:
	default:
		// todo: log full
	}
}
