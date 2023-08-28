package iouringnet

import (
	"encoding/binary"
	"log"
)

func init() {
	log.SetFlags(log.Flags() | log.Lshortfile)
}

type TlvPacket struct {
	Tag     uint32
	Payload []byte
}

func packetTlv(packet *TlvPacket) []byte {
	msg := make([]byte, 8+len(packet.Payload))
	binary.LittleEndian.PutUint32(msg, packet.Tag)
	binary.LittleEndian.PutUint32(msg, uint32(len(packet.Payload)))
	copy(msg, packet.Payload)
	return msg
}
