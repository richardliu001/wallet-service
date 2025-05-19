package http

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// LoggingMiddleware prints request/response metrics.
func LoggingMiddleware(log *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Infof("%s %s %d %s",
			c.Request.Method, c.Request.URL.Path, c.Writer.Status(), time.Since(start))
	}
}

// RateLimitMiddleware simple token bucket per IP.
func RateLimitMiddleware(rps, burst int) gin.HandlerFunc {
	var mu sync.Mutex
	buckets := make(map[string]*rate.Limiter)
	newLimiter := func() *rate.Limiter { return rate.NewLimiter(rate.Limit(rps), burst) }
	return func(c *gin.Context) {
		ip, _, _ := net.SplitHostPort(c.Request.RemoteAddr)
		mu.Lock()
		lim, ok := buckets[ip]
		if !ok {
			lim = newLimiter()
			buckets[ip] = lim
		}
		mu.Unlock()
		if !lim.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}
		c.Next()
	}
}
