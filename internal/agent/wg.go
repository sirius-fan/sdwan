package agent

import (
	"fmt"
	"net"
	"os/exec"
	"strings"

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
		// Optional endpoint from controller (host:port). If resolvable, set it.
		if p.Endpoint != "" {
			if ep, err := net.ResolveUDPAddr("udp", p.Endpoint); err == nil {
				pc.Endpoint = ep
			}
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

// ensureWGDevice creates the interface if not present and sets it up.
func ensureWGDevice(iface string) error {
	// Check if link exists: ip link show dev iface
	if err := exec.Command("ip", "link", "show", "dev", iface).Run(); err != nil {
		// Try to add
		if err := exec.Command("ip", "link", "add", iface, "type", "wireguard").Run(); err != nil {
			return fmt.Errorf("add wg dev: %w", err)
		}
	}
	// Set up
	if err := exec.Command("ip", "link", "set", "dev", iface, "up").Run(); err != nil {
		return fmt.Errorf("link up: %w", err)
	}
	return nil
}

// ensureWGAddrRoute assigns the /32 address and adds on-link route for CIDR if provided.
func ensureWGAddrRoute(iface, selfIP, networkCIDR string) error {
	if selfIP != "" {
		// Remove existing /32s for idempotency (best-effort)
		_ = exec.Command("ip", "addr", "flush", "dev", iface).Run()
		// Assign /32
		if err := exec.Command("ip", "addr", "add", fmt.Sprintf("%s/32", selfIP), "dev", iface).Run(); err != nil {
			return fmt.Errorf("addr add: %w", err)
		}
	}
	if networkCIDR != "" {
		// Ensure route exists; if already present, ignore
		out, _ := exec.Command("ip", "route", "show", "dev", iface).Output()
		if !strings.Contains(string(out), networkCIDR) {
			_ = exec.Command("ip", "route", "add", networkCIDR, "dev", iface).Run()
		}
	}
	return nil
}

// gatherLocalEndpoints collects local candidate endpoints based on local addresses.
func gatherLocalEndpoints(listenPort int) []string {
	if listenPort <= 0 {
		return nil
	}
	var eps []string
	ifaces, _ := net.Interfaces()
	for _, ifc := range ifaces {
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() || ip.IsUnspecified() {
				continue
			}
			if ip.To4() == nil {
				// prefer IPv4 for initial simplicity
				continue
			}
			eps = append(eps, net.JoinHostPort(ip.String(), fmt.Sprintf("%d", listenPort)))
		}
	}
	// Deduplicate simple
	m := map[string]struct{}{}
	var out []string
	for _, e := range eps {
		if _, ok := m[e]; ok {
			continue
		}
		m[e] = struct{}{}
		out = append(out, e)
	}
	return out
}
