package middleware

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// RedisRateLimiter implementa uma janela fixa simples (Fixed Window) baseada em Redis.
func RedisRateLimiter(redisClient *redis.Client, maxRequests int64, window time.Duration, logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := getIP(r)
			path := r.URL.Path

			key := fmt.Sprintf("rate_limit:%s:%s", path, ip)

			ctx := context.Background()
			
			// Incrementa o contador
			count, err := redisClient.Incr(ctx, key).Result()
			if err != nil {
				logger.Error("Failed to increment rate limit counter", zap.Error(err), zap.String("key", key))
				// Em caso de erro no Redis, podemos escolher liberar ou bloquear. 
				// Fail-open para manter disponibilidade, porém registramos no log.
				next.ServeHTTP(w, r)
				return
			}

			// Se for a primeira requisição na janela, seta o TTL
			if count == 1 {
				redisClient.Expire(ctx, key, window)
			}

			// Bloqueia caso exceda
			if count > maxRequests {
				logger.Warn("Rate limit exceeded", zap.String("ip", ip), zap.String("path", path), zap.Int64("count", count))
				http.Error(w, "Too many requests", http.StatusTooManyRequests)
				return
			}

			// Chama o próximo handler se estiver liberado
			next.ServeHTTP(w, r)
		})
	}
}

// getIP tenta resgatar o IP original caso esteja atrás de Proxies (Nginx, ELB, etc)
func getIP(r *http.Request) string {
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		ips := strings.Split(forwarded, ",")
		return strings.TrimSpace(ips[0])
	}
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
