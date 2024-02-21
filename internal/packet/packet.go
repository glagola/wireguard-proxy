package packet

import "net"

type Packet struct {
	Addr net.UDPAddr
	Data []byte
}

type UDPConnection struct {
	local  net.UDPAddr
	remote net.UDPAddr
}
