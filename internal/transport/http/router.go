package http

import (
	"github.com/gin-gonic/gin"
	"github.com/richardliu001/wallet-service/internal/config"
	"github.com/richardliu001/wallet-service/internal/service"
	"go.uber.org/zap"
)

type rateCfg struct {
	RPS   int
	Burst int
}

func NewRouter(svc *service.WalletService, rl config.RateLimitConfig, log *zap.SugaredLogger) *gin.Engine {
	r := gin.New()
	r.Use(LoggingMiddleware(log))
	r.Use(RateLimitMiddleware(rl.RPS, rl.Burst))
	RegisterHandlers(r, svc)
	return r
}
