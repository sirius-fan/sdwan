package controller

import (
	"errors"
	"net"
	"sort"
	"sync"
	"time"

	"sdwan/internal/common"
)

type Store struct {
	mu       sync.RWMutex
	net      *net.IPNet
	baseIP   net.IP
	nextHost uint32
	nodes    map[string]common.Node // by ID
	byIP     map[string]string      // ip -> id
}

func NewStore(cidr string) (*Store, error) {
	ip, n, err := common.ValidateCIDR(cidr)
	if err != nil {
		return nil, err
	}
	return &Store{
		net:      n,
		baseIP:   ip.Mask(n.Mask),
		nextHost: 10, // start from .10 to keep some room
		nodes:    map[string]common.Node{},
		byIP:     map[string]string{},
	}, nil
}

func (s *Store) allocateIP() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := 0; i < 1<<16; i++ {
		ip := incIP(s.baseIP, s.nextHost)
		s.nextHost++
		if !s.net.Contains(ip) {
			continue
		}
		sip := ip.String()
		if _, used := s.byIP[sip]; used {
			continue
		}
		return sip, nil
	}
	return "", errors.New("no free ip")
}

func incIP(base net.IP, off uint32) net.IP {
	b := make(net.IP, len(base))
	copy(b, base)
	// assume IPv4 for starter
	bi := uint32(b[12])<<24 | uint32(b[13])<<16 | uint32(b[14])<<8 | uint32(b[15])
	bi += off
	b[12] = byte(bi >> 24)
	b[13] = byte(bi >> 16)
	b[14] = byte(bi >> 8)
	b[15] = byte(bi)
	return b
}

func (s *Store) UpsertNode(n common.Node) (common.Node, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// ensure IP reserved
	if n.TunnelIP == "" {
		ip, err := s.allocateIPNoLock()
		if err != nil {
			return n, err
		}
		n.TunnelIP = ip
	}
	// choose a preferred endpoint when available (simple: pick first valid ip:port)
	if n.Endpoint == "" {
		for _, e := range n.Endpoints {
			if _, _, err := net.SplitHostPort(e); err == nil {
				n.Endpoint = e
				break
			}
		}
	}
	n.LastSeen = time.Now()
	s.nodes[n.ID] = n
	s.byIP[n.TunnelIP] = n.ID
	return n, nil
}

// UpdateEndpoints updates a node's endpoints and selects a preferred endpoint.
func (s *Store) UpdateEndpoints(nodeID string, eps []string, listenPort int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, ok := s.nodes[nodeID]
	if !ok {
		return errors.New("node not found")
	}
	n.Endpoints = eps
	n.Endpoint = ""
	for _, e := range eps {
		if _, _, err := net.SplitHostPort(e); err == nil {
			n.Endpoint = e
			break
		}
	}
	// Optionally, if no explicit endpoints but we know listen port, we could infer from recent request's remote addr (not available here).
	s.nodes[nodeID] = n
	return nil
}

func (s *Store) allocateIPNoLock() (string, error) {
	for i := 0; i < 1<<16; i++ {
		ip := incIP(s.baseIP, s.nextHost)
		s.nextHost++
		if !s.net.Contains(ip) {
			continue
		}
		sip := ip.String()
		if _, used := s.byIP[sip]; used {
			continue
		}
		return sip, nil
	}
	return "", errors.New("no free ip")
}

func (s *Store) ListPeers(exceptID string) []common.Node {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make([]common.Node, 0, len(s.nodes))
	for id, n := range s.nodes {
		if id == exceptID {
			continue
		}
		res = append(res, n)
	}
	// stable order by TunnelIP
	sort.Slice(res, func(i, j int) bool { return res[i].TunnelIP < res[j].TunnelIP })
	return res
}
