package controller

import (
	"encoding/json"
	"log"
	"net/http"

	"sdwan/internal/common"
)

type HTTPServer struct {
	store *Store
}

func NewHTTPServer(store *Store) *HTTPServer {
	return &HTTPServer{store: store}
}

func (s *HTTPServer) RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/api/register", s.handleRegister)
	mux.HandleFunc("/api/peers", s.handlePeers)
	mux.HandleFunc("/api/announce", s.handleAnnounce)
}

func (s *HTTPServer) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req common.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	priv, pub, err := common.KeyPair()
	if err != nil {
		log.Printf("keypair: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	id, err := common.RandomID(12)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	n := common.Node{
		ID:        id,
		Hostname:  req.Hostname,
		OS:        req.OS,
		Version:   req.Version,
		PublicKey: pub,
		Endpoints: req.Endpoints,
	}
	n, err = s.store.UpsertNode(n)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	resp := common.RegisterResponse{
		Node:        n,
		Peers:       s.store.ListPeers(n.ID),
		PrivKey:     priv,
		NetworkCIDR: s.store.net.String(),
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *HTTPServer) handlePeers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	// For demo, return all peers (no auth)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(common.PeerUpdate{Peers: s.store.ListPeers("")})
}

func (s *HTTPServer) handleAnnounce(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req common.AnnounceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	// Update node's endpoints and recompute chosen Endpoint
	if err := s.store.UpdateEndpoints(req.NodeID, req.Endpoints, req.ListenPort); err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
