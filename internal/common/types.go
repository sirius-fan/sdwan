package common

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// Node represents a participating agent in the mesh.
type Node struct {
	ID        string    `json:"id"`
	Hostname  string    `json:"hostname"`
	OS        string    `json:"os"`
	Version   string    `json:"version"`
	PublicKey string    `json:"publicKey"`
	TunnelIP  string    `json:"tunnelIp"`
	Endpoints []string  `json:"endpoints"`
	Relays    []string  `json:"relays"`
	LastSeen  time.Time `json:"lastSeen"`
}

// RegisterRequest is sent by agent to controller to join.
type RegisterRequest struct {
	Hostname  string   `json:"hostname"`
	OS        string   `json:"os"`
	Version   string   `json:"version"`
	Endpoints []string `json:"endpoints"` // candidate public endpoints (ip:port)
}

// RegisterResponse contains allocated identity and peers.
type RegisterResponse struct {
	Node     Node    `json:"node"`
	Peers    []Node  `json:"peers"`
	PrivKey  string  `json:"privKey"` // base64-encoded or wg key string
	RelayUDP *string `json:"relayUdp,omitempty"`
}

// PeerUpdate periodically sent by controller to agents.
type PeerUpdate struct {
	Peers []Node `json:"peers"`
}

// KeyPair generates a WireGuard keypair as strings.
func KeyPair() (priv string, pub string, err error) {
	k, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return "", "", err
	}
	pk := k.PublicKey()
	return k.String(), pk.String(), nil
}

// RandomID returns URL-safe random ID.
func RandomID(n int) (string, error) {
	if n <= 0 {
		return "", errors.New("length must be > 0")
	}
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// ValidateCIDR checks CIDR and returns ip and network.
func ValidateCIDR(cidr string) (ip net.IP, network *net.IPNet, err error) {
	ip, network, err = net.ParseCIDR(cidr)
	return
}
