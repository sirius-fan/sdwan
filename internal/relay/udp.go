package relay

import (
	"log"
	"net"
)

type UDPRelay struct {
	addr string
}

func NewUDPRelay(addr string) *UDPRelay { return &UDPRelay{addr: addr} }

func (u *UDPRelay) Run() error {
	conn, err := net.ListenPacket("udp", u.addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	log.Printf("relay listening on %s", u.addr)
	buf := make([]byte, 64*1024)
	for {
		n, src, err := conn.ReadFrom(buf)
		if err != nil {
			return err
		}
		// Echo back for now (placeholder for TURN-like behavior)
		_, _ = conn.WriteTo(buf[:n], src)
	}
}
