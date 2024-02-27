package connection

import (
	"context"
	"fmt"
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

	clientAddr net.UDPAddr
}

func New(toClient, toServer *net.UDPConn, clientAddr net.UDPAddr) Route {
	return Route{
		fromClientToServer: make(chan packet.Packet, queueSize),
		fromServerToClient: make(chan packet.Packet, queueSize),
		toClient:           toClient,
		toServer:           toServer,
		clientAddr:         clientAddr,
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

				// TODO read deadline
				n, _, err := c.toServer.ReadFromUDP(buffer)
				if err != nil {
					log.Fatalf("Serve: Error during udp reading %e", err)
				}

				if n > 0 {
					// fmt.Printf("From server: %s\n", string(buffer[:n]))

					c.fromServerToClient <- packet.Packet{
						Addr: c.clientAddr,
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
				// fmt.Println("Packet received CLIENT -> SERVER")
				// fmt.Printf("Packet from %s forwarded to server\n", c.clientAddr.String())

				n, err := c.toServer.Write(packet.Data)
				// TODO set write deadline
				if err != nil {
					fmt.Println("ERR: case packet := <-c.fromClientToServer:")
					panic(err)
				}

				if n != len(packet.Data) {
					fmt.Printf("C->S: failed to send all %d / %d\n", n, len(packet.Data))
				}

			case packet := <-c.fromServerToClient:
				// fmt.Println("Packet received SERVER -> CLIENT")
				// fmt.Printf("Server responded to client %s\n", c.clientAddr.String())

				n, err := c.toClient.WriteToUDP(packet.Data, &packet.Addr)
				// TODO set write deadline
				if err != nil {
					fmt.Println("ERR:case packet := <-c.fromServerToClient: ")
					panic(err)
				}

				if n != len(packet.Data) {
					fmt.Printf("S->C: failed to send all %d / %d\n", n, len(packet.Data))
				}
			}
		}
	}()
}
