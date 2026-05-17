package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/netip"

	"github.com/gin-gonic/gin"

	"erg.ninja/internal/dto/response"
)

type FirewallService interface {
	IsIPBlocked(ctx context.Context, ip string) (bool, error)
}

func FirewallMiddleware(svc FirewallService) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if isLoopbackIP(ip) {
			c.Next()
			return
		}

		blocked, err := svc.IsIPBlocked(c.Request.Context(), ip)
		if err != nil {
			// If redis is down, we might want to allow or deny. Usually allow but log.
			c.Next()
			return
		}

		if blocked {
			response.ErrorGin(c, http.StatusForbidden, "IP_BLOCKED", fmt.Sprintf("Access denied for your IP: %s", ip))
			c.Abort()
			return
		}

		c.Next()
	}
}

func isLoopbackIP(ip string) bool {
	addr, err := netip.ParseAddr(ip)
	return err == nil && addr.IsLoopback()
}
