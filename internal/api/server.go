package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Prasang-money/distributedCounter/internal/counter"
	"github.com/Prasang-money/distributedCounter/internal/discovery"
	"github.com/Prasang-money/distributedCounter/models"
	"github.com/gin-gonic/gin"
)

type Server struct {
	counter    *counter.Counter
	discovery  *discovery.Discovery
	httpServer *http.Server
}

func NewServer(port string, counter *counter.Counter, discovery *discovery.Discovery) *Server {
	server := Server{
		counter:   counter,
		discovery: discovery,
	}

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: server.SetupRoutes(),
	}
	server.httpServer = httpServer
	return &server
}

// Start starts the server.
func (s *Server) Start() error {

	return s.httpServer.ListenAndServe()
}

// Stop stops the server.
func (s *Server) Stop() error {

	ctxShutDown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.httpServer.Shutdown(ctxShutDown)
}

func (s *Server) SetupRoutes() *gin.Engine {
	r := gin.Default()
	// Define your routes here
	r.GET("/health", s.handleHealth)
	r.GET("/count", s.handleGetCount)
	r.POST("/count/increment", s.handleIncrementCount)
	r.GET("/peers", s.handleGetPeers)
	r.POST(discovery.DiscoveryPath, s.handleDiscovery)
	r.POST(counter.PropagationPath, s.handlePropagateCount)

	return r
}

func (s *Server) handleHealth(c *gin.Context) {
	health := map[string]interface{}{
		"status":    "healthy",
		"node_id":   s.discovery.GetNodeID(),
		"count":     s.counter.GetValue(),
		"num_peers": len(s.discovery.GetPeers()),
	}
	c.JSON(200, health)
}

func (s *Server) handleGetCount(c *gin.Context) {

	response := models.CountResponse{
		Count:  s.counter.GetValue(),
		NodeID: s.discovery.GetNodeID(),
	}
	c.JSON(http.StatusOK, response)
}

func (s *Server) handleIncrementCount(c *gin.Context) {
	incrementID, newValue, err := s.counter.Increment()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to increment count"})
		return
	}

	response := models.IncrementResponse{
		IncrementID: incrementID,
		NewValue:    newValue,
	}
	c.JSON(http.StatusOK, response)

}

func (s *Server) handleGetPeers(c *gin.Context) {

	response := models.PeersResponse{
		Peers: s.discovery.GetPeers(),
	}
	c.IndentedJSON(http.StatusOK, response)

}

func (s *Server) handleDiscovery(c *gin.Context) {
	var msg models.DiscoveryMessage
	if err := c.BindJSON(&msg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	peerList := s.discovery.HandleDiscoveryMessage(msg)

	if msg.Type == models.JoinMessage && peerList != nil {
		res := &models.DiscoveryResponse{
			Peers: peerList,
			Value: s.counter.GetValue(),
		}
		c.IndentedJSON(http.StatusOK, res)
	} else {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}

}

func (s *Server) handlePropagateCount(c *gin.Context) {

	msg := models.IncrementPropagation{}
	if err := c.BindJSON(&msg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if err := s.counter.HandleIncrementMessage(msg); err != nil {
		log.Printf("[API] Error handling increment message: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to handle increment propagation"})
		return
	}

	c.JSON(http.StatusOK, gin.H{})

}
