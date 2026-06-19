package main

import (
	"bufio"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

type BlocklistManager struct {
	db             *DB
	blockedDomains map[string]bool
	allowedDomains map[string]bool
	mu             sync.RWMutex
	isUpdating     bool
}

func NewBlocklistManager(db *DB) *BlocklistManager {
	return &BlocklistManager{
		db:             db,
		blockedDomains: make(map[string]bool),
		allowedDomains: make(map[string]bool),
	}
}

// LoadRules loads custom whitelist/blacklist rules and blocklists from DB
func (bm *BlocklistManager) LoadRules() error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	bm.blockedDomains = make(map[string]bool)
	bm.allowedDomains = make(map[string]bool)

	// Load custom rules
	rules, err := bm.db.GetCustomRules()
	if err != nil {
		return fmt.Errorf("failed to load custom rules: %w", err)
	}

	for _, rule := range rules {
		domain := normalizeDomain(rule.Domain)
		if domain == "" {
			continue
		}
		if rule.RuleType == "ALLOW" {
			bm.allowedDomains[domain] = true
		} else if rule.RuleType == "BLOCK" {
			bm.blockedDomains[domain] = true
		}
	}

	return nil
}

// UpdateBlocklists downloads all enabled lists and stores their rules
func (bm *BlocklistManager) UpdateBlocklists() error {
	bm.mu.Lock()
	if bm.isUpdating {
		bm.mu.Unlock()
		return fmt.Errorf("update already in progress")
	}
	bm.isUpdating = true
	bm.mu.Unlock()

	defer func() {
		bm.mu.Lock()
		bm.isUpdating = false
		bm.mu.Unlock()
	}()

	lists, err := bm.db.GetLists()
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 30 * time.Second}

	for _, list := range lists {
		if !list.Enabled {
			continue
		}

		fmt.Printf("Updating list: %s (%s)\n", list.Name, list.URL)
		resp, err := client.Get(list.URL)
		if err != nil {
			fmt.Printf("Error downloading %s: %v\n", list.URL, err)
			continue
		}

		scanner := bufio.NewScanner(resp.Body)
		count := 0

		localBlocked := make(map[string]bool)

		for scanner.Scan() {
			line := scanner.Text()
			domain := parseHostsLine(line)
			if domain != "" && !isSystemDomain(domain) {
				localBlocked[domain] = true
				count++
			}
		}
		resp.Body.Close()

		bm.db.UpdateListCount(list.ID, count)

		bm.mu.Lock()
		for d := range localBlocked {
			if !bm.allowedDomains[d] {
				bm.blockedDomains[d] = true
			}
		}
		bm.mu.Unlock()

		fmt.Printf("List %s updated: %d domains loaded\n", list.Name, count)
	}

	return nil
}

// IsBlocked checks if a domain (or its parent domains) is blocked
func (bm *BlocklistManager) IsBlocked(domain string) bool {
	domain = normalizeDomain(domain)
	if domain == "" {
		return false
	}

	bm.mu.RLock()
	defer bm.mu.RUnlock()

	parts := strings.Split(domain, ".")
	
	// Check whitelists first (specific to general)
	for i := 0; i < len(parts); i++ {
		parent := strings.Join(parts[i:], ".")
		if bm.allowedDomains[parent] {
			return false
		}
	}

	// Check blacklists (specific to general)
	for i := 0; i < len(parts); i++ {
		parent := strings.Join(parts[i:], ".")
		if bm.blockedDomains[parent] {
			return true
		}
	}

	return false
}

func parseHostsLine(line string) string {
	line = strings.TrimSpace(line)
	if idx := strings.Index(line, "#"); idx != -1 {
		line = line[:idx]
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}

	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}

	if len(fields) >= 2 {
		ip := fields[0]
		if ip == "127.0.0.1" || ip == "0.0.0.0" || ip == "::1" {
			return normalizeDomain(fields[1])
		}
	}

	if len(fields) == 1 {
		return normalizeDomain(fields[0])
	}

	return ""
}

func normalizeDomain(domain string) string {
	domain = strings.ToLower(strings.TrimSpace(domain))
	domain = strings.TrimSuffix(domain, ".")
	return domain
}

func isSystemDomain(domain string) bool {
	systemDomains := map[string]bool{
		"localhost":             true,
		"localhost.localdomain": true,
		"local":                 true,
		"broadcasthost":         true,
		"ip6-localhost":         true,
		"ip6-loopback":          true,
	}
	return systemDomains[domain]
}
