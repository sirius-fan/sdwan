package agent

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"sdwan/internal/common"
)

type Config struct {
	Controller string // http base, e.g. http://127.0.0.1:8080
	Hostname   string
	OS         string
	Version    string
	Endpoints  []string // public candidates
	Iface      string   // wireguard interface name, e.g. wg0
	ListenPort int      // wg listen port, 0 for kernel-chosen
}

type Agent struct {
	cfg    Config
	self   *common.Node
	priv   string
	client *http.Client
}

func New(cfg Config) *Agent {
	return &Agent{cfg: cfg, client: &http.Client{Timeout: 10 * time.Second}}
}

func (a *Agent) Register() error {
	body := common.RegisterRequest{
		Hostname:  a.cfg.Hostname,
		OS:        a.cfg.OS,
		Version:   a.cfg.Version,
		Endpoints: a.cfg.Endpoints,
	}
	b, _ := json.Marshal(body)
	resp, err := a.client.Post(a.cfg.Controller+"/api/register", "application/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("register status %d", resp.StatusCode)
	}
	var r common.RegisterResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return err
	}
	a.self = &r.Node
	a.priv = r.PrivKey
	return nil
}

func (a *Agent) FetchPeers() ([]common.Node, error) {
	if a.self == nil {
		return nil, errors.New("not registered")
	}
	resp, err := a.client.Get(a.cfg.Controller + "/api/peers")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("peers status %d", resp.StatusCode)
	}
	var u common.PeerUpdate
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, err
	}
	return u.Peers, nil
}

func (a *Agent) PrivateKey() string { return a.priv }
func (a *Agent) Self() *common.Node { return a.self }

// ApplyWireGuard applies WireGuard device and peer configuration. Safe to call repeatedly.
func (a *Agent) ApplyWireGuard(peers []common.Node) error {
	if a.self == nil || a.priv == "" {
		return errors.New("not registered")
	}
	if a.cfg.Iface == "" {
		a.cfg.Iface = "wg0"
	}
	return applyWG(a.cfg.Iface, a.cfg.ListenPort, a.Self(), a.priv, peers)
}
