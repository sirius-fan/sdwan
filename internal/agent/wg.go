package agent

import (
	"fmt"
	"net"

	"sdwan/internal/common"

	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// applyWG ensures a WireGuard device exists and applies self key/ip and peers.
// For simplicity we expect the OS to have a wg interface created externally if required.
func applyWG(iface string, listenPort int, self *common.Node, priv string, peers []common.Node) error {
	client, err := wgctrl.New()
	if err != nil {
		return err
	}
	defer client.Close()

	// Parse private key
	pk, err := wgtypes.ParseKey(priv)
	if err != nil {
		return fmt.Errorf("invalid private key: %w", err)
	}

	// Build peer configs
	peerCfgs := make([]wgtypes.PeerConfig, 0, len(peers))
	for _, p := range peers {
		if p.PublicKey == "" || p.TunnelIP == "" {
			continue
		}
		pub, err := wgtypes.ParseKey(p.PublicKey)
		if err != nil {
			continue
		}
		allowed := []net.IPNet{}
		if ip := net.ParseIP(p.TunnelIP); ip != nil {
			mask := net.CIDRMask(32, 32)
			allowed = append(allowed, net.IPNet{IP: ip, Mask: mask})
		}
		pc := wgtypes.PeerConfig{
			PublicKey:         pub,
			ReplaceAllowedIPs: true,
			AllowedIPs:        allowed,
		}
		peerCfgs = append(peerCfgs, pc)
	}

	cfg := wgtypes.Config{
		PrivateKey:   &pk,
		ListenPort:   intPtr(listenPort),
		ReplacePeers: true,
		Peers:        peerCfgs,
	}
	return client.ConfigureDevice(iface, cfg)
}

func intPtr(v int) *int { return &v }
