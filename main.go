package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os/signal"
	"strings"
	"syscall"

	"github.com/Prasang-money/distributedCounter/internal/api"
	"github.com/Prasang-money/distributedCounter/internal/counter"
	"github.com/Prasang-money/distributedCounter/internal/discovery"
)

func main() {
	var (
		nodeID    = flag.String("id", "", "Unique node ID (e.g., localhost:8080)")
		port      = flag.String("port", "8080", "Port to listen on")
		peersFlag = flag.String("peers", "", "Comma-separated list of bootstrap peers (e.g., localhost:8081,localhost:8082)")
	)
	flag.Parse()

	if *nodeID == "" {
		*nodeID = fmt.Sprintf("localhost:%s", *port)
	}

	var peers []string
	if *peersFlag != "" {
		peers = strings.Split(*peersFlag, ",")
		for i, peer := range peers {
			peers[i] = strings.TrimSpace(peer)
		}
	}
	// Create a synchronization channel on joining the discovery and counter moduless
	syncChan := make(chan int64, 1)
	discovery := discovery.NewDiscovery(*nodeID, syncChan)

	if err := discovery.Start(peers); err != nil {
		log.Fatalf("Failed to start discovery: %v", err)
	}
	counter := counter.NewCounter(discovery, syncChan)
	server := api.NewServer(*port, counter, discovery)

	log.Printf("Discovery service started with peers: %v", peers)

	go func() {

		if err := server.Start(); err != nil {
			fmt.Printf("Failed to start server: %v\n", err)
		}

	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()
	stop()
	discovery.Stop()
	counter.Stop()
	if err := server.Stop(); err != nil {
		log.Fatalf("server forced to shutdown: %v", err)
	}

	log.Println("Server exited properly")
}
