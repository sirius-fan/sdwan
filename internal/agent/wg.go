package agent

import (
	"bytes"
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
	if err := run(ipCmd(), "link", "show", "dev", iface); err != nil {
		// Try to add
		if err := run(ipCmd(), "link", "add", iface, "type", "wireguard"); err != nil {
			return fmt.Errorf("add wg dev: %w", err)
		}
	}
	// Set up
	if err := run(ipCmd(), "link", "set", "dev", iface, "up"); err != nil {
		return fmt.Errorf("link up: %w", err)
	}
	return nil
}

// ensureWGAddrRoute assigns the /32 address and adds on-link route for CIDR if provided.
func ensureWGAddrRoute(iface, selfIP, networkCIDR string) error {
	if selfIP != "" {
		// Remove existing /32s for idempotency (best-effort)
		_ = run(ipCmd(), "addr", "flush", "dev", iface)
		// Assign /32
		if err := run(ipCmd(), "addr", "add", fmt.Sprintf("%s/32", selfIP), "dev", iface); err != nil {
			return fmt.Errorf("addr add: %w", err)
		}
	}
	if networkCIDR != "" {
		// Ensure route exists; if already present, ignore
		out, _ := runOut(ipCmd(), "route", "show", "dev", iface)
		if !strings.Contains(string(out), networkCIDR) {
			_ = run(ipCmd(), "route", "add", networkCIDR, "dev", iface)
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

// ipCmd tries to find an absolute path to `ip` for environments where /sbin is not in PATH.
func ipCmd() string {
	if p, err := exec.LookPath("ip"); err == nil {
		return p
	}
	candidates := []string{"/usr/sbin/ip", "/sbin/ip", "/usr/bin/ip"}
	for _, c := range candidates {
		if _, err := exec.LookPath(c); err == nil {
			return c
		}
	}
	// fallback to "ip" and let the error surface if truly missing
	return "ip"
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		if b := buf.String(); b != "" {
			return fmt.Errorf("%v: %s", err, strings.TrimSpace(b))
		}
		return err
	}
	return nil
}

func runOut(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return buf.Bytes(), err
	}
	return buf.Bytes(), nil
}
