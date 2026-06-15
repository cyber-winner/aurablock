package main

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/miekg/dns"
)

type CacheItem struct {
	Msg       *dns.Msg
	ExpiresAt time.Time
}

type DNSCache struct {
	items map[string]CacheItem
	mu    sync.RWMutex
}

func NewDNSCache() *DNSCache {
	c := &DNSCache{items: make(map[string]CacheItem)}
	go func() {
		for {
			time.Sleep(1 * time.Minute)
			c.Cleanup()
		}
	}()
	return c
}

func (c *DNSCache) Get(key string) (*dns.Msg, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	item, found := c.items[key]
	if !found {
		return nil, false
	}
	if time.Now().After(item.ExpiresAt) {
		return nil, false
	}
	return item.Msg.Copy(), true
}

func (c *DNSCache) Set(key string, msg *dns.Msg, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = CacheItem{
		Msg:       msg.Copy(),
		ExpiresAt: time.Now().Add(ttl),
	}
}

func (c *DNSCache) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	for k, v := range c.items {
		if now.After(v.ExpiresAt) {
			delete(c.items, k)
		}
	}
}

type DNSServer struct {
	Addr       string
	bm         *BlocklistManager
	db         *DB
	cache      *DNSCache
	upstreams  []string
	udpServer  *dns.Server
	tcpServer  *dns.Server
	pubSub     *LogPubSub
	enabled    bool
	mu         sync.RWMutex
}

func NewDNSServer(addr string, bm *BlocklistManager, db *DB, pubSub *LogPubSub) *DNSServer {
	return &DNSServer{
		Addr:      addr,
		bm:        bm,
		db:        db,
		cache:     NewDNSCache(),
		pubSub:    pubSub,
		upstreams: []string{"1.1.1.1:53", "8.8.8.8:53"},
		enabled:   true,
	}
}

func (s *DNSServer) Start() error {
	dns.HandleFunc(".", s.handleDNSRequest)

	s.udpServer = &dns.Server{Addr: s.Addr, Net: "udp"}
	s.tcpServer = &dns.Server{Addr: s.Addr, Net: "tcp"}

	errChan := make(chan error, 2)

	go func() {
		fmt.Printf("Starting UDP DNS server on %s...\n", s.Addr)
		if err := s.udpServer.ListenAndServe(); err != nil {
			errChan <- fmt.Errorf("UDP server failed: %w", err)
		}
	}()

	go func() {
		fmt.Printf("Starting TCP DNS server on %s...\n", s.Addr)
		if err := s.tcpServer.ListenAndServe(); err != nil {
			errChan <- fmt.Errorf("TCP server failed: %w", err)
		}
	}()

	select {
	case err := <-errChan:
		return err
	case <-time.After(1 * time.Second):
		return nil
	}
}

func (s *DNSServer) Stop() {
	if s.udpServer != nil {
		s.udpServer.Shutdown()
	}
	if s.tcpServer != nil {
		s.tcpServer.Shutdown()
	}
}

func (s *DNSServer) SetEnabled(enabled bool) {
	s.mu.Lock()
	s.enabled = enabled
	s.mu.Unlock()
}

func (s *DNSServer) IsEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.enabled
}

func (s *DNSServer) SetUpstreams(upstreams []string) {
	s.mu.Lock()
	s.upstreams = upstreams
	s.mu.Unlock()
}

func (s *DNSServer) handleDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
	start := time.Now()
	m := new(dns.Msg)
	m.SetReply(r)
	m.Compress = false

	if len(r.Question) == 0 {
		w.WriteMsg(m)
		return
	}

	q := r.Question[0]
	domain := normalizeDomain(q.Name)
	clientIP, _, _ := net.SplitHostPort(w.RemoteAddr().String())

	s.mu.RLock()
	enabled := s.enabled
	upstreams := s.upstreams
	s.mu.RUnlock()

	blocked := false
	if enabled {
		blocked = s.bm.IsBlocked(domain)
	}

	var status string
	var answerIP string
	var responseMsg *dns.Msg
	var err error

	if blocked {
		status = "BLOCKED"
		m.Authoritative = true
		
		switch q.Qtype {
		case dns.TypeA:
			rr, _ := dns.NewRR(fmt.Sprintf("%s 3600 IN A 0.0.0.0", q.Name))
			m.Answer = append(m.Answer, rr)
			answerIP = "0.0.0.0"
		case dns.TypeAAAA:
			rr, _ := dns.NewRR(fmt.Sprintf("%s 3600 IN AAAA ::", q.Name))
			m.Answer = append(m.Answer, rr)
			answerIP = "::"
		}
		
		responseMsg = m
	} else {
		status = "ALLOWED"
		
		cacheKey := fmt.Sprintf("%s-%d", domain, q.Qtype)
		if cachedMsg, found := s.cache.Get(cacheKey); found {
			cachedMsg.Id = r.Id
			w.WriteMsg(cachedMsg)
			
			elapsed := time.Since(start).Milliseconds()
			var cachedIP string
			if len(cachedMsg.Answer) > 0 {
				cachedIP = getAnswerIP(cachedMsg.Answer[0])
			}
			
			log := QueryLog{
				Timestamp:      time.Now(),
				Domain:         domain,
				QueryType:      dns.TypeToString[q.Qtype],
				ClientIP:       clientIP,
				Status:         "ALLOWED (CACHE)",
				Answer:         cachedIP,
				ResponseTimeMs: elapsed,
			}
			s.db.LogQuery(&log)
			if s.pubSub != nil {
				s.pubSub.Publish(&log)
			}
			return
		}

		responseMsg, err = s.queryUpstream(r, upstreams)
		if err != nil {
			m.SetRcode(r, dns.RcodeServerFailure)
			w.WriteMsg(m)
			
			elapsed := time.Since(start).Milliseconds()
			log := QueryLog{
				Timestamp:      time.Now(),
				Domain:         domain,
				QueryType:      dns.TypeToString[q.Qtype],
				ClientIP:       clientIP,
				Status:         "ERROR",
				Answer:         err.Error(),
				ResponseTimeMs: elapsed,
			}
			s.db.LogQuery(&log)
			if s.pubSub != nil {
				s.pubSub.Publish(&log)
			}
			return
		}

		if len(responseMsg.Answer) > 0 {
			answerIP = getAnswerIP(responseMsg.Answer[0])
			ttl := time.Duration(responseMsg.Answer[0].Header().Ttl) * time.Second
			if ttl > 0 {
				s.cache.Set(cacheKey, responseMsg, ttl)
			}
		}
	}

	w.WriteMsg(responseMsg)

	elapsed := time.Since(start).Milliseconds()
	log := QueryLog{
		Timestamp:      time.Now(),
		Domain:         domain,
		QueryType:      dns.TypeToString[q.Qtype],
		ClientIP:       clientIP,
		Status:         status,
		Answer:         answerIP,
		ResponseTimeMs: elapsed,
	}
	s.db.LogQuery(&log)
	if s.pubSub != nil {
		s.pubSub.Publish(&log)
	}
}

func (s *DNSServer) queryUpstream(r *dns.Msg, upstreams []string) (*dns.Msg, error) {
	c := new(dns.Client)
	c.Timeout = 3 * time.Second

	var lastErr error
	for _, upstream := range upstreams {
		reply, _, err := c.Exchange(r, upstream)
		if err == nil {
			return reply, nil
		}
		lastErr = err
	}

	return nil, fmt.Errorf("upstreams failed: %v", lastErr)
}

func getAnswerIP(rr dns.RR) string {
	switch record := rr.(type) {
	case *dns.A:
		return record.A.String()
	case *dns.AAAA:
		return record.AAAA.String()
	case *dns.CNAME:
		return record.Target
	default:
		return rr.String()
	}
}
