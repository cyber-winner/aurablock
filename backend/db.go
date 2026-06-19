package main

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "modernc.org/sqlite"
)

type QueryLog struct {
	ID             int64     `json:"id"`
	Timestamp      time.Time `json:"timestamp"`
	Domain         string    `json:"domain"`
	QueryType      string    `json:"query_type"`
	ClientIP       string    `json:"client_ip"`
	Status         string    `json:"status"` // ALLOWED or BLOCKED
	Answer         string    `json:"answer"`
	ResponseTimeMs int64     `json:"response_time_ms"`
}

type BlockList struct {
	ID          int64     `json:"id"`
	URL         string    `json:"url"`
	Name        string    `json:"name"`
	Enabled     bool      `json:"enabled"`
	LastUpdated time.Time `json:"last_updated"`
	ItemCount   int       `json:"item_count"`
}

type CustomRule struct {
	ID       int64  `json:"id"`
	Domain   string `json:"domain"`
	RuleType string `json:"rule_type"` // BLOCK or ALLOW
	Comment  string `json:"comment"`
}

type DB struct {
	*sql.DB
}

func InitDB(dbPath string) (*DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Create tables
	queries := []string{
		`CREATE TABLE IF NOT EXISTS logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			domain TEXT,
			query_type TEXT,
			client_ip TEXT,
			status TEXT,
			answer TEXT,
			response_time_ms INTEGER
		);`,
		`CREATE TABLE IF NOT EXISTS lists (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			url TEXT UNIQUE,
			name TEXT,
			enabled INTEGER DEFAULT 1,
			last_updated DATETIME,
			item_count INTEGER DEFAULT 0
		);`,
		`CREATE TABLE IF NOT EXISTS custom_rules (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			domain TEXT UNIQUE,
			rule_type TEXT, -- BLOCK or ALLOW
			comment TEXT
		);`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return nil, fmt.Errorf("failed to execute migrations: %w", err)
		}
	}

	// Insert default lists if empty
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM lists").Scan(&count)
	if err == nil && count == 0 {
		defaultLists := []struct {
			URL  string
			Name string
		}{
			{"https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts", "StevenBlack Hosts (Unified Ad/Tracker List)"},
			{"https://adaway.org/hosts.txt", "AdAway Blocklist"},
			{"https://v.firebog.net/hosts/AdguardDNS.txt", "AdGuard DNS Filter (Firebog)"},
			{"https://v.firebog.net/hosts/Easyprivacy.txt", "EasyPrivacy (Firebog)"},
		}
		for _, dl := range defaultLists {
			db.Exec("INSERT INTO lists (url, name, enabled, last_updated) VALUES (?, ?, 1, ?)", dl.URL, dl.Name, time.Time{})
		}
	}

	return &DB{db}, nil
}

func (db *DB) LogQuery(log *QueryLog) error {
	_, err := db.Exec(
		"INSERT INTO logs (domain, query_type, client_ip, status, answer, response_time_ms) VALUES (?, ?, ?, ?, ?, ?)",
		log.Domain, log.QueryType, log.ClientIP, log.Status, log.Answer, log.ResponseTimeMs,
	)
	return err
}

func (db *DB) GetRecentLogs(limit int, statusFilter string) ([]QueryLog, error) {
	var rows *sql.Rows
	var err error
	if statusFilter != "" {
		rows, err = db.Query("SELECT id, timestamp, domain, query_type, client_ip, status, answer, response_time_ms FROM logs WHERE status = ? ORDER BY id DESC LIMIT ?", statusFilter, limit)
	} else {
		rows, err = db.Query("SELECT id, timestamp, domain, query_type, client_ip, status, answer, response_time_ms FROM logs ORDER BY id DESC LIMIT ?", limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []QueryLog
	for rows.Next() {
		var log QueryLog
		var ts string
		err := rows.Scan(&log.ID, &ts, &log.Domain, &log.QueryType, &log.ClientIP, &log.Status, &log.Answer, &log.ResponseTimeMs)
		if err != nil {
			return nil, err
		}
		log.Timestamp, _ = time.Parse("2006-01-02 15:04:05", ts)
		if log.Timestamp.IsZero() {
			log.Timestamp, _ = time.Parse(time.RFC3339, ts)
		}
		logs = append(logs, log)
	}
	return logs, nil
}

func (db *DB) GetStats() (map[string]interface{}, error) {
	var totalQueries int64
	var blockedQueries int64
	var dbSize int64

	db.QueryRow("SELECT COUNT(*) FROM logs").Scan(&totalQueries)
	db.QueryRow("SELECT COUNT(*) FROM logs WHERE status = 'BLOCKED'").Scan(&blockedQueries)

	fileInfo, err := os.Stat("aurablock.db")
	if err == nil {
		dbSize = fileInfo.Size()
	}

	var blocklistDomains int64
	db.QueryRow("SELECT SUM(item_count) FROM lists WHERE enabled = 1").Scan(&blocklistDomains)

	var percentBlocked float64
	if totalQueries > 0 {
		percentBlocked = float64(blockedQueries) / float64(totalQueries) * 100
	}

	hourlyQuery := `
		SELECT strftime('%Y-%m-%dT%H:00:00Z', timestamp) as hour, COUNT(*) as total, SUM(CASE WHEN status='BLOCKED' THEN 1 ELSE 0 END) as blocked
		FROM logs
		WHERE timestamp >= datetime('now', '-24 hours')
		GROUP BY hour
		ORDER BY hour ASC
	`
	rows, err := db.Query(hourlyQuery)
	var hourlyData []map[string]interface{}
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var hour string
			var total, blocked int
			if err := rows.Scan(&hour, &total, &blocked); err == nil {
				hourlyData = append(hourlyData, map[string]interface{}{
					"hour":    hour,
					"total":   total,
					"blocked": blocked,
				})
			}
		}
	}

	topBlockedQuery := `
		SELECT domain, COUNT(*) as count
		FROM logs
		WHERE status = 'BLOCKED'
		GROUP BY domain
		ORDER BY count DESC
		LIMIT 10
	`
	tRows, err := db.Query(topBlockedQuery)
	var topBlocked []map[string]interface{}
	if err == nil {
		defer tRows.Close()
		for tRows.Next() {
			var domain string
			var count int
			if err := tRows.Scan(&domain, &count); err == nil {
				topBlocked = append(topBlocked, map[string]interface{}{
					"domain": domain,
					"count":  count,
				})
			}
		}
	}

	topQueriedQuery := `
		SELECT domain, COUNT(*) as count
		FROM logs
		GROUP BY domain
		ORDER BY count DESC
		LIMIT 10
	`
	qRows, err := db.Query(topQueriedQuery)
	var topQueried []map[string]interface{}
	if err == nil {
		defer qRows.Close()
		for qRows.Next() {
			var domain string
			var count int
			if err := qRows.Scan(&domain, &count); err == nil {
				topQueried = append(topQueried, map[string]interface{}{
					"domain": domain,
					"count":  count,
				})
			}
		}
	}

	return map[string]interface{}{
		"total_queries":     totalQueries,
		"blocked_queries":   blockedQueries,
		"percent_blocked":   percentBlocked,
		"blocklist_domains": blocklistDomains,
		"db_size_bytes":     dbSize,
		"hourly_data":       hourlyData,
		"top_blocked":       topBlocked,
		"top_queried":       topQueried,
	}, nil
}

func (db *DB) GetLists() ([]BlockList, error) {
	rows, err := db.Query("SELECT id, url, name, enabled, last_updated, item_count FROM lists")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lists []BlockList
	for rows.Next() {
		var list BlockList
		var ts string
		if err := rows.Scan(&list.ID, &list.URL, &list.Name, &list.Enabled, &ts, &list.ItemCount); err != nil {
			return nil, err
		}
		list.LastUpdated, _ = time.Parse("2006-01-02 15:04:05", ts)
		if list.LastUpdated.IsZero() {
			list.LastUpdated, _ = time.Parse(time.RFC3339, ts)
		}
		lists = append(lists, list)
	}
	return lists, nil
}

func (db *DB) AddList(url, name string) error {
	_, err := db.Exec("INSERT INTO lists (url, name, enabled, last_updated, item_count) VALUES (?, ?, 1, ?, 0)", url, name, time.Time{})
	return err
}

func (db *DB) DeleteList(id int64) error {
	_, err := db.Exec("DELETE FROM lists WHERE id = ?", id)
	return err
}

func (db *DB) ToggleList(id int64, enabled bool) error {
	val := 0
	if enabled {
		val = 1
	}
	_, err := db.Exec("UPDATE lists SET enabled = ? WHERE id = ?", val, id)
	return err
}

func (db *DB) UpdateListCount(id int64, count int) error {
	_, err := db.Exec("UPDATE lists SET item_count = ?, last_updated = ? WHERE id = ?", count, time.Now(), id)
	return err
}

func (db *DB) GetCustomRules() ([]CustomRule, error) {
	rows, err := db.Query("SELECT id, domain, rule_type, comment FROM custom_rules")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []CustomRule
	for rows.Next() {
		var rule CustomRule
		if err := rows.Scan(&rule.ID, &rule.Domain, &rule.RuleType, &rule.Comment); err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func (db *DB) AddCustomRule(domain, ruleType, comment string) error {
	_, err := db.Exec("INSERT OR REPLACE INTO custom_rules (domain, rule_type, comment) VALUES (?, ?, ?)", domain, ruleType, comment)
	return err
}

func (db *DB) DeleteCustomRule(id int64) error {
	_, err := db.Exec("DELETE FROM custom_rules WHERE id = ?", id)
	return err
}
