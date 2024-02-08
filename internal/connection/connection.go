package connection

import (
	"context"
	"log"
	"net"
	"wireguard-proxy/internal/packet"
)

const queueSize = 1024

type Route struct {
	fromClientToServer chan packet.Packet
	fromServerToClient chan packet.Packet

	toClient *net.UDPConn
	toServer *net.UDPConn
}

func New(toClient, toServer *net.UDPConn) Route {
	return Route{
		fromClientToServer: make(chan packet.Packet, queueSize),
		fromServerToClient: make(chan packet.Packet, queueSize),
		toClient:           toClient,
		toServer:           toServer,
	}
}

func (c Route) ForwardToServer(p packet.Packet) {
	c.fromClientToServer <- p
}

func (c Route) Serve(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				buffer := make([]byte, 1300)

				n, _, err := c.toServer.ReadFromUDP(buffer)
				if err != nil {
					log.Fatalf("Error during udp reading %e", err)
				}

				if n > 0 {
					c.fromServerToClient <- packet.Packet{
						Conn: c.toServer,
						Data: buffer[:n],
					}
				}
			}
		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				c.toServer.Close()
				c.toClient.Close()
				return
			case packet := <-c.fromClientToServer:
				c.toServer.Write(packet.Data)
			case packet := <-c.fromServerToClient:
				c.toClient.Write(packet.Data)
			}
		}
	}()
}
