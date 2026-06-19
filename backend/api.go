package main

import (
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type APIServer struct {
	db        *DB
	dnsServer *DNSServer
	bm        *BlocklistManager
	port      int
	pubSub    *LogPubSub
}

type LogPubSub struct {
	subs map[chan *QueryLog]bool
	mu   sync.RWMutex
}

func NewLogPubSub() *LogPubSub {
	return &LogPubSub{
		subs: make(map[chan *QueryLog]bool),
	}
}

func (ps *LogPubSub) Subscribe() chan *QueryLog {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ch := make(chan *QueryLog, 100)
	ps.subs[ch] = true
	return ch
}

func (ps *LogPubSub) Unsubscribe(ch chan *QueryLog) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	delete(ps.subs, ch)
	close(ch)
}

func (ps *LogPubSub) Publish(log *QueryLog) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	for ch := range ps.subs {
		select {
		case ch <- log:
		default:
			// Buffer full, skip
		}
	}
}

func NewAPIServer(port int, db *DB, dnsServer *DNSServer, bm *BlocklistManager, pubSub *LogPubSub) *APIServer {
	return &APIServer{
		port:      port,
		db:        db,
		dnsServer: dnsServer,
		bm:        bm,
		pubSub:    pubSub,
	}
}

func (s *APIServer) Start() error {
	_ = mime.AddExtensionType(".crx", "application/x-chrome-extension")
	mux := http.NewServeMux()

	corsHandler := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			fmt.Printf("[HTTP] %s %s from %s\n", r.Method, r.URL.Path, r.RemoteAddr)
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			next(w, r)
		}
	}

	mux.HandleFunc("GET /api/stats", corsHandler(s.handleGetStats))
	mux.HandleFunc("GET /api/history", corsHandler(s.handleGetHistory))
	mux.HandleFunc("GET /api/lists", corsHandler(s.handleGetLists))
	mux.HandleFunc("POST /api/lists", corsHandler(s.handleAddList))
	mux.HandleFunc("DELETE /api/lists/{id}", corsHandler(s.handleDeleteList))
	mux.HandleFunc("POST /api/lists/{id}/toggle", corsHandler(s.handleToggleList))
	
	mux.HandleFunc("GET /api/rules", corsHandler(s.handleGetRules))
	mux.HandleFunc("POST /api/rules", corsHandler(s.handleAddRule))
	mux.HandleFunc("DELETE /api/rules/{id}", corsHandler(s.handleDeleteRule))
	
	mux.HandleFunc("GET /api/status", corsHandler(s.handleGetStatus))
	mux.HandleFunc("POST /api/toggle", corsHandler(s.handleToggleBlocking))
	mux.HandleFunc("POST /api/update", corsHandler(s.handleUpdateBlocklists))
	mux.HandleFunc("POST /api/upstreams", corsHandler(s.handleSetUpstreams))
	
	mux.HandleFunc("GET /api/logs/stream", corsHandler(s.handleLogsStream))

	fs := http.FileServer(http.Dir("./dist"))
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api") {
			// If file doesn't exist under dist, serve index.html for SPA routing
			if _, err := http.Dir("./dist").Open(r.URL.Path); err != nil {
				http.ServeFile(w, r, "./dist/index.html")
				return
			}
		}
		fs.ServeHTTP(w, r)
	}))

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: corsHandler(mux.ServeHTTP),
	}

	fmt.Printf("Starting HTTP API server on port %d...\n", s.port)
	return server.ListenAndServe()
}

func (s *APIServer) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.db.GetStats()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, stats)
}

func (s *APIServer) handleGetHistory(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}
	status := r.URL.Query().Get("status")
	logs, err := s.db.GetRecentLogs(limit, status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, logs)
}

func (s *APIServer) handleGetLists(w http.ResponseWriter, r *http.Request) {
	lists, err := s.db.GetLists()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, lists)
}

func (s *APIServer) handleAddList(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL  string `json:"url"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if body.URL == "" || body.Name == "" {
		http.Error(w, "url and name are required", http.StatusBadRequest)
		return
	}

	err := s.db.AddList(body.URL, body.Name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, map[string]string{"status": "success"})
}

func (s *APIServer) handleDeleteList(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	err = s.db.DeleteList(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, map[string]string{"status": "success"})
}

func (s *APIServer) handleToggleList(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = s.db.ToggleList(id, body.Enabled)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, map[string]string{"status": "success"})
}

func (s *APIServer) handleGetRules(w http.ResponseWriter, r *http.Request) {
	rules, err := s.db.GetCustomRules()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, rules)
}

func (s *APIServer) handleAddRule(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Domain   string `json:"domain"`
		RuleType string `json:"rule_type"`
		Comment  string `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if body.Domain == "" || (body.RuleType != "BLOCK" && body.RuleType != "ALLOW") {
		http.Error(w, "domain and valid rule_type (BLOCK/ALLOW) are required", http.StatusBadRequest)
		return
	}

	err := s.db.AddCustomRule(body.Domain, body.RuleType, body.Comment)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.bm.LoadRules()
	respondJSON(w, map[string]string{"status": "success"})
}

func (s *APIServer) handleDeleteRule(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	err = s.db.DeleteCustomRule(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.bm.LoadRules()
	respondJSON(w, map[string]string{"status": "success"})
}

func (s *APIServer) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	s.dnsServer.mu.RLock()
	status := map[string]interface{}{
		"blocking_enabled": s.dnsServer.enabled,
		"upstreams":        s.dnsServer.upstreams,
		"is_updating":      s.bm.isUpdating,
		"version":          "1.0.0",
	}
	s.dnsServer.mu.RUnlock()
	respondJSON(w, status)
}

func (s *APIServer) handleToggleBlocking(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.dnsServer.SetEnabled(body.Enabled)
	respondJSON(w, map[string]interface{}{"status": "success", "blocking_enabled": body.Enabled})
}

func (s *APIServer) handleUpdateBlocklists(w http.ResponseWriter, r *http.Request) {
	go func() {
		err := s.bm.UpdateBlocklists()
		if err != nil {
			fmt.Printf("Error updating lists: %v\n", err)
		}
	}()
	respondJSON(w, map[string]string{"status": "updating"})
}

func (s *APIServer) handleSetUpstreams(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Upstreams []string `json:"upstreams"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(body.Upstreams) == 0 {
		http.Error(w, "at least one upstream is required", http.StatusBadRequest)
		return
	}

	var cleaned []string
	for _, u := range body.Upstreams {
		u = strings.TrimSpace(u)
		if !strings.Contains(u, ":") {
			u = u + ":53"
		}
		cleaned = append(cleaned, u)
	}

	s.dnsServer.SetUpstreams(cleaned)
	respondJSON(w, map[string]interface{}{"status": "success", "upstreams": cleaned})
}

func (s *APIServer) handleLogsStream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	logChan := s.pubSub.Subscribe()
	defer s.pubSub.Unsubscribe(logChan)

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	cn := r.Context()

	for {
		select {
		case log := <-logChan:
			data, err := json.Marshal(log)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			w.(http.Flusher).Flush()
		case <-ticker.C:
			fmt.Fprintf(w, ": ping\n\n")
			w.(http.Flusher).Flush()
		case <-cn.Done():
			return
		}
	}
}

func respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
