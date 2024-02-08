package packet

import "net"

type Packet struct {
	Conn *net.UDPConn
	Data []byte
}
