package main

import (
	"algorithm/iouringnet"
	"encoding/binary"
	"fmt"
	"log"
	"math/rand"
	"net"
	"syscall"
	"time"
)

type connector struct {
}

func newConnector(fd int, addr *syscall.SockaddrInet4) iouringnet.UserConnector {
	c := new(connector)
	return c
}

func (c *connector) String() string {
	return ""
}
func (c *connector) PreHandleRequest(request *iouringnet.TlvPacket) (response *iouringnet.TlvPacket, userData interface{}, err error) {
	//log.Printf("[server] pre handle request:%v\n", request.Tag)
	return nil, nil, nil
}
func (c *connector) HandleRequest(request *iouringnet.TlvPacket, userData interface{}) (response *iouringnet.TlvPacket, newUserData interface{}, err error) {
	log.Printf("[server] handle request:%v, with payload:%v\n", request.Tag, string(request.Payload))
	resp := &iouringnet.TlvPacket{
		Tag:     request.Tag,
		Payload: request.Payload,
	}
	return resp, nil, nil
}

func startServer(addr string) {
	l, err := iouringnet.NewListener(addr, newConnector)
	if err != nil {
		panic(err)
	}
	err = l.Start()
	if err != nil {
		panic(err)
	}
}

func startClient(no int, addr string) {
	c, err := net.Dial("tcp", addr)
	if err != nil {
		panic(err)
	}

	for {
		time.Sleep(time.Second * time.Duration(rand.Int31n(10)+1))

		payload := []byte(fmt.Sprintf("%v say hello", no))
		msg := make([]byte, 8+len(payload))
		binary.BigEndian.PutUint32(msg, uint32(123))
		binary.BigEndian.PutUint32(msg[4:], uint32(len(payload)))
		copy(msg[8:], payload)

		_, err = c.Write(msg)
		if err != nil {
			panic(err)
		}

		n, err := c.Read(msg)
		if err != nil {
			panic(err)
		}
		log.Printf("[client] client[%v] recv from server msg:%v\n", no, string(msg[:n]))
	}

}

func main() {
	go startServer(":8888")
	time.Sleep(time.Second * 2)
	for i := 1; i <= 10; i++ {
		no := i
		go startClient(no, ":8888")
	}
	select {}
}
