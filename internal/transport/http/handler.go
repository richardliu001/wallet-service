package http

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/richardliu001/wallet-service/internal/service"
	"github.com/shopspring/decimal"
)

func RegisterHandlers(r *gin.Engine, svc *service.WalletService) {
	v1 := r.Group("/v1")
	{
		v1.POST("/wallets/:id/deposit", depositHandler(svc))
		v1.POST("/wallets/:id/withdraw", withdrawHandler(svc))
		v1.POST("/wallets/:id/transfer", transferHandler(svc))
		v1.GET("/wallets/:id/balance", balanceHandler(svc))
		v1.GET("/wallets/:id/history", historyHandler(svc))
	}
}

type depositReq struct {
	Amount         string `json:"amount" binding:"required"`
	IdempotencyKey string `json:"idempotency_key" binding:"required"`
}

func depositHandler(svc *service.WalletService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req depositReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		amt, err := decimal.NewFromString(req.Amount)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid amount"})
			return
		}
		bal, err := svc.Deposit(c, id, amt, req.IdempotencyKey)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"balance": bal})
	}
}

type withdrawReq struct {
	Amount         string `json:"amount" binding:"required"`
	IdempotencyKey string `json:"idempotency_key" binding:"required"`
}

func withdrawHandler(svc *service.WalletService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req withdrawReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		amt, err := decimal.NewFromString(req.Amount)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid amount"})
			return
		}
		bal, err := svc.Withdraw(c, id, amt, req.IdempotencyKey)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"balance": bal})
	}
}

type transferReq struct {
	ToID           string `json:"to_id" binding:"required"`
	Amount         string `json:"amount" binding:"required"`
	IdempotencyKey string `json:"idempotency_key" binding:"required"`
}

func transferHandler(svc *service.WalletService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req transferReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		fromID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		toID, err := strconv.ParseUint(req.ToID, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid to_id"})
			return
		}
		amt, err := decimal.NewFromString(req.Amount)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid amount"})
			return
		}
		fromBal, toBal, err := svc.Transfer(c, fromID, toID, amt, req.IdempotencyKey)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"from_balance": fromBal, "to_balance": toBal})
	}
}

func balanceHandler(svc *service.WalletService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		bal, err := svc.GetBalance(c, id)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"balance": bal})
	}
}
func historyHandler(svc *service.WalletService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
		sinceStr := c.DefaultQuery("since", time.Now().Add(-24*time.Hour).Format(time.RFC3339))
		since, err := time.Parse(time.RFC3339, sinceStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid since"})
			return
		}
		txs, err := svc.GetHistory(c, id, limit, since)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, txs)
	}
}
