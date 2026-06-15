package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	dnsAddr := flag.String("dns-addr", "127.0.0.1:53", "DNS server bind address")
	apiPort := flag.Int("api-port", 8082, "HTTP API server port")
	dbPath := flag.String("db-path", "aurablock.db", "SQLite database file path")
	flag.Parse()

	fmt.Println("==================================================")
	fmt.Println("             AuraBlock - Core Engine              ")
	fmt.Println("==================================================")

	db, err := InitDB(*dbPath)
	if err != nil {
		log.Fatalf("Fatal: Database initialization failed: %v", err)
	}
	defer db.Close()
	fmt.Println("[+] Database initialized successfully.")

	bm := NewBlocklistManager(db)
	err = bm.LoadRules()
	if err != nil {
		log.Printf("[!] Warning: Failed to load rules: %v\n", err)
	} else {
		fmt.Println("[+] Custom rules loaded into memory.")
	}

	lists, err := db.GetLists()
	if err == nil {
		totalCount := 0
		for _, l := range lists {
			if l.Enabled {
				totalCount += l.ItemCount
			}
		}
		if totalCount == 0 {
			fmt.Println("[*] Blocklist database is empty. Triggering background download...")
			go func() {
				if err := bm.UpdateBlocklists(); err != nil {
					log.Printf("[!] Error updating blocklists: %v\n", err)
				} else {
					fmt.Println("[+] Background blocklist download complete.")
				}
			}()
		} else {
			fmt.Printf("[+] Loaded %d blocked domains from cache.\n", totalCount)
			go func() {
				if err := bm.UpdateBlocklists(); err != nil {
					log.Printf("[!] Error updating blocklists: %v\n", err)
				}
			}()
		}
	}

	pubSub := NewLogPubSub()

	dnsServer := NewDNSServer(*dnsAddr, bm, db, pubSub)
	err = dnsServer.Start()
	if err != nil {
		log.Fatalf("Fatal: DNS Server failed to start: %v\nNote: Binding to port 53 requires privileges. Try running with sudo, or use: setcap 'cap_net_bind_service=+ep' <binary>", err)
	}
	fmt.Printf("[+] DNS Server listening on %s (UDP/TCP)\n", *dnsAddr)

	apiServer := NewAPIServer(*apiPort, db, dnsServer, bm, pubSub)
	go func() {
		if err := apiServer.Start(); err != nil {
			log.Fatalf("Fatal: HTTP API Server failed: %v", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	fmt.Printf("\n[*] Received signal %v. Shutting down gracefully...\n", sig)
	dnsServer.Stop()
	fmt.Println("[+] AuraBlock stopped.")
}
