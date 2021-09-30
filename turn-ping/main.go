// NB this originaly came from:
//    	https://github.com/pion/turn/blob/13867664acbcf7a2b55f561fc4ed61b46638438d/examples/turn-client/tcp/main.go
//    	https://github.com/pion/turn/blob/13867664acbcf7a2b55f561fc4ed61b46638438d/examples/turn-client/udp/main.go
//     	https://github.com/pion/turn/tree/13867664acbcf7a2b55f561fc4ed61b46638438d/examples#turn-client

package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/pion/logging"
	"github.com/pion/turn/v2"
)

func main() {
	host := flag.String("host", "", "TURN Server name.")
	port := flag.Int("port", 3478, "Listening port.")
	protocol := flag.String("protocol", "tcp", "Protocol (e.g. tcp or udp; defaults to tcp).")
	user := flag.String("user", "", "A pair of username and password (e.g. user=pass).")
	realm := flag.String("realm", "coturn", "Realm (defaults to coturn).")
	count := flag.Int("count", 0, "Number of pings to send. (defaults to 0 (infinite)).")
	flag.Parse()

	if len(*host) == 0 {
		log.Fatalf("'host' is required")
	}

	if len(*user) == 0 {
		log.Fatalf("'user' is required")
	}

	cred := strings.SplitN(*user, "=", 2)

	turnServerAddr := fmt.Sprintf("%s:%d", *host, *port)

	var conn net.PacketConn

	if *protocol == "tcp" {
		// Dial TURN Server
		tcpConn, err := net.Dial("tcp", turnServerAddr)
		if err != nil {
			panic(err)
		}
		// wrap net.Conn in a STUNConn.
		// This allows us to simulate datagram based communication over a net.Conn
		conn = turn.NewSTUNConn(tcpConn)
	} else {
		// TURN client won't create a local listening socket by itself.
		udpConn, err := net.ListenPacket("udp4", "0.0.0.0:0")
		if err != nil {
			panic(err)
		}
		defer func() {
			if closeErr := udpConn.Close(); closeErr != nil {
				panic(closeErr)
			}
		}()
		conn = udpConn
	}

	// Start a new TURN Client.
	cfg := &turn.ClientConfig{
		STUNServerAddr: turnServerAddr,
		TURNServerAddr: turnServerAddr,
		Conn:           conn,
		Username:       cred[0],
		Password:       cred[1],
		Realm:          *realm,
		LoggerFactory:  logging.NewDefaultLoggerFactory(),
	}

	client, err := turn.NewClient(cfg)
	if err != nil {
		panic(err)
	}
	defer client.Close()

	// Start listening on the conn provided.
	err = client.Listen()
	if err != nil {
		panic(err)
	}

	// Allocate a relay socket on the TURN server. On success, it
	// will return a net.PacketConn which represents the remote
	// socket.
	relayConn, err := client.Allocate()
	if err != nil {
		panic(err)
	}
	defer func() {
		if closeErr := relayConn.Close(); closeErr != nil {
			panic(closeErr)
		}
	}()

	// The relayConn's local address is actually the transport
	// address assigned on the TURN server.
	log.Printf("relayed-address=%s", relayConn.LocalAddr().String())

	// Perform a ping test agaist the relayConn we have just allocated.
	err = doPingTest(*count, client, relayConn)
	if err != nil {
		panic(err)
	}
}

func doPingTest(count int, client *turn.Client, relayConn net.PacketConn) error {
	// Send BindingRequest to learn our external IP
	mappedAddr, err := client.SendBindingRequest()
	if err != nil {
		return err
	}

	log.Printf("mapped-address=%s", mappedAddr)
	log.Println("Press Ctrl+C to stop")

	// Set up pinger socket (pingerConn)
	pingerConn, err := net.ListenPacket("udp4", "0.0.0.0:0")
	if err != nil {
		panic(err)
	}
	defer func() {
		if closeErr := pingerConn.Close(); closeErr != nil {
			panic(closeErr)
		}
	}()

	// Punch a UDP hole for the relayConn by sending a data to the mappedAddr.
	// This will trigger a TURN client to generate a permission request to the
	// TURN server. After this, packets from the IP address will be accepted by
	// the TURN server.
	_, err = relayConn.WriteTo([]byte("Hello"), mappedAddr)
	if err != nil {
		return err
	}

	// Start read-loop on pingerConn
	go func() {
		buf := make([]byte, 1600)
		for {
			n, from, pingerErr := pingerConn.ReadFrom(buf)
			if pingerErr != nil {
				break
			}

			msg := string(buf[:n])
			if sentAt, pingerErr := time.Parse(time.RFC3339Nano, msg); pingerErr == nil {
				rtt := time.Since(sentAt)
				log.Printf("%d bytes from from %s time=%d ms\n", n, from.String(), int(rtt.Seconds()*1000))
			}
		}
	}()

	// Start read-loop on relayConn
	go func() {
		buf := make([]byte, 1600)
		for {
			n, from, readerErr := relayConn.ReadFrom(buf)
			if readerErr != nil {
				break
			}

			// Echo back
			if _, readerErr = relayConn.WriteTo(buf[:n], from); readerErr != nil {
				break
			}
		}
	}()

	time.Sleep(500 * time.Millisecond)

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(c)

	// Send packets from relayConn to the echo server until the user presses Ctrl+C.
	t := count
pingLoop:
	for {
		msg := time.Now().Format(time.RFC3339Nano)
		_, err = pingerConn.WriteTo([]byte(msg), relayConn.LocalAddr())
		if err != nil {
			return err
		}

		// For simplicity, this example does not wait for the pong (reply).
		// Instead, sleep 1 second.
		select {
		case <-c:
			break pingLoop
		case <-time.After(1 * time.Second):
		}

		if count > 0 {
			t--
			if t == 0 {
				break
			}
		}
	}

	log.Println("Bye...")
	return nil
}