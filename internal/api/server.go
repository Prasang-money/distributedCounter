package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/Prasang-money/distributedCounter/internal/counter"
	"github.com/gin-gonic/gin"
)

type Server struct {
	counter    *counter.Counter
	httpServer *http.Server
}

func NewServer(port string, counter *counter.Counter) *Server {
	server := Server{
		counter: counter,
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
	r.POST("/discovery", s.handleDiscovery)
	r.POST("/count/propagate", s.handlePropagateCount)

	return r
}

func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(200, gin.H{
		"status": "ok",
	})
}

func (s *Server) handleGetCount(c *gin.Context) {

}

func (s *Server) handleIncrementCount(c *gin.Context) {

}

func (s *Server) handleGetPeers(c *gin.Context) {

}

func (s *Server) handleDiscovery(c *gin.Context) {

}

func (s *Server) handlePropagateCount(c *gin.Context) {

}
