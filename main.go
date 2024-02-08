package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"wireguard-proxy/internal/connection"
	"wireguard-proxy/internal/packet"
)

type Config struct {
	proxyPort int

	serverPort int
	serverIP   string
}

func (cfg Config) MustAddrForClients() (res *net.UDPAddr) {
	return mustNewUDPAddr("", cfg.proxyPort)
}

func (cfg Config) MustServerAddr() (res *net.UDPAddr) {
	return mustNewUDPAddr(cfg.serverIP, cfg.serverPort)
}

func mustNewUDPAddr(ip string, port int) (res *net.UDPAddr) {
	ipAddr := fmt.Sprintf("%s:%d", ip, port)

	var err error
	if res, err = net.ResolveUDPAddr("udp", ipAddr); err != nil {
		log.Fatal(err)
	}

	return
}

func main() {
	cfg := &Config{
		proxyPort: 51820,

		serverPort: 1000,
		serverIP:   "localhost",
	}

	serverAddr := cfg.MustServerAddr()

	ctx, _ := gracefullShutdown()

	packetsFromClients := udpToChan(ctx, cfg.MustAddrForClients())

	byClient := make(map[string]connection.Route)

	for p := range packetsFromClients {
		clientAddr := p.Conn.RemoteAddr().String()
		log.Printf("New packet from %s\n", clientAddr)

		conn, exists := byClient[clientAddr]

		if !exists {
			proxyToServerConn, err := newServerConn(ctx, serverAddr)
			if err != nil {
				log.Printf("Failed to create separate connection to server for %s. Packet of size = %d bytes dropped.\n", clientAddr, len(p.Data))
				continue
			}

			log.Printf("New connection %s <-> %d:proxy:%s <-> %d:%s\n", clientAddr, cfg.proxyPort, strings.Split(proxyToServerConn.LocalAddr().String(), ":")[1], cfg.serverPort, cfg.serverIP)

			byClient[clientAddr] = connection.New(
				p.Conn,
				proxyToServerConn,
			)

			byClient[clientAddr].Serve(ctx)
		}

		conn.ForwardToServer(p)
	}
}

func newServerConn(ctx context.Context, serverAddr *net.UDPAddr) (*net.UDPConn, error) {
	localAddr, err := net.ResolveUDPAddr("udp", ":0")
	if err != nil {
		panic(err)
	}

	conn, err := net.DialUDP("udp", localAddr, serverAddr) // TODO add context
	if err != nil {
		panic(err)
	}

	return conn, nil
}

func gracefullShutdown() (ctx context.Context, cancel context.CancelFunc) {
	ctx, cancel = context.WithCancel(context.Background())

	go func() {
		exit := make(chan os.Signal, 1)
		signal.Notify(exit, os.Interrupt, syscall.SIGTERM)

		select {
		case <-ctx.Done():
		case _signal := <-exit:
			log.Printf("Caught signal: %s", _signal)
			log.Println("Shutting down")
			cancel()
		}
	}()

	return
}

func udpToChan(ctx context.Context, receiverAddr *net.UDPAddr) chan packet.Packet {
	packets := make(chan packet.Packet, 10000)

	go func() {
		socket, err := net.ListenUDP("udp", receiverAddr)
		if err != nil {
			log.Fatalf("Failed to listen %s", receiverAddr)
		}
		defer socket.Close()
		defer close(packets)

		stop := make(chan struct{})
		defer func() {
			stop <- struct{}{}
		}()

		done := false
		go func() {
			select {
			case <-ctx.Done():
				done = true
			case <-stop:
			}
		}()

		for !done {
			buffer := make([]byte, 1300) // TODO alloc memory only if previous spent

			n, senderAddr, err := socket.ReadFromUDP(buffer)
			if err != nil {
				log.Fatalf("Error during udp reading %e", err)
			}

			if n > 0 {
				conn, _ := net.DialUDP("udp", receiverAddr, senderAddr) // TODO add context
				packets <- packet.Packet{
					Conn: conn,
					Data: buffer[:n],
				}
			}
		}
	}()

	return packets
}
