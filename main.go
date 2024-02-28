package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"wireguard-proxy/internal/connection"
	"wireguard-proxy/internal/logger"
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

	localAddr := cfg.MustAddrForClients()
	clientToProxyConn := mustListenUDP(localAddr)
	packetsFromClients := udpToChan(ctx, clientToProxyConn)

	byClient := make(map[string]connection.Route)

	for p := range packetsFromClients {
		clientAddr := p.Addr.String()

		conn, exists := byClient[clientAddr]

		if !exists {
			proxyToServerConn, err := newServerConn(ctx, serverAddr)
			if err != nil {
				log.Printf("Failed to create separate connection to server for %s. Packet of size = %d bytes dropped.\n", clientAddr, len(p.Data))
				continue
			}

			log.Printf("New connection %s <-> %d:proxy:%s <-> %d:%s\n", clientAddr, cfg.proxyPort, strings.Split(proxyToServerConn.LocalAddr().String(), ":")[1], cfg.serverPort, cfg.serverIP)

			byClient[clientAddr] = connection.New(
				clientToProxyConn,
				proxyToServerConn,
				p.Addr,
			)

			byClient[clientAddr].Serve(ctx)

			conn = byClient[clientAddr]
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
			log.Printf("Caught signal: %s\n", _signal)
			log.Println("Shutting down")
			cancel()
		}
	}()

	return
}

func udpToChan(ctx context.Context, socket *net.UDPConn) chan packet.Packet {
	packets := make(chan packet.Packet, 10000)

	ctxWithLogger := logger.AddLogger(ctx, slog.Default())

	go func() {
		udpPipe(ctxWithLogger, socket, func(data []byte, sender net.UDPAddr) {
			packets <- packet.Packet{
				Data: data,
				Addr: sender,
			}
		})
	}()

	return packets
}

func udpPipe(ctx context.Context, socket *net.UDPConn, pipe func([]byte, net.UDPAddr)) {
	logger := logger.MustGetLogger(ctx)

	buffer := make([]byte, 65507)
	for ctx.Err() == nil {
		socket.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		n, senderAddr, err := socket.ReadFromUDP(buffer)
		if err != nil {
			if e, ok := err.(net.Error); ok && e.Timeout() {
				continue
			}

			logger.Error("failed to read from UDP %e", err, slog.String("sender", senderAddr.String()))
		}

		if n <= 0 {
			continue
		}

		pipe(
			buffer[:n],
			*senderAddr,
		)
		buffer = make([]byte, 65507)
	}
}

func mustListenUDP(addr *net.UDPAddr) *net.UDPConn {
	socket, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatalf("Failed to listen %s", addr)
	}

	return socket
}
