package antispam

import (
	"context"
	"sync"
	"time"
)

// RateLimiter 速率限制器
type RateLimiter struct {
	ipLimits   map[string]*TokenBucket
	userLimits map[string]*TokenBucket
	mu         sync.RWMutex
}

// NewRateLimiter 创建速率限制器
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		ipLimits:   make(map[string]*TokenBucket),
		userLimits: make(map[string]*TokenBucket),
	}
}

// CheckIP 检查 IP 速率限制
func (r *RateLimiter) CheckIP(ip string, limit int, window time.Duration) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	bucket, ok := r.ipLimits[ip]
	if !ok {
		bucket = NewTokenBucket(limit, window)
		r.ipLimits[ip] = bucket
	}

	return bucket.Allow()
}

// CheckUser 检查用户速率限制
func (r *RateLimiter) CheckUser(user string, limit int, window time.Duration) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	bucket, ok := r.userLimits[user]
	if !ok {
		bucket = NewTokenBucket(limit, window)
		r.userLimits[user] = bucket
	}

	return bucket.Allow()
}

// Cleanup 清理过期的限制器
func (r *RateLimiter) Cleanup(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.mu.Lock()
			// 清理长时间未使用的限制器
			now := time.Now()
			for ip, bucket := range r.ipLimits {
				if now.Sub(bucket.lastAccess) > 24*time.Hour {
					delete(r.ipLimits, ip)
				}
			}
			for user, bucket := range r.userLimits {
				if now.Sub(bucket.lastAccess) > 24*time.Hour {
					delete(r.userLimits, user)
				}
			}
			r.mu.Unlock()
		case <-ctx.Done():
			return
		}
	}
}

// TokenBucket 令牌桶
type TokenBucket struct {
	capacity   int
	refillRate time.Duration
	tokens     int
	lastRefill time.Time
	lastAccess time.Time
	mu         sync.Mutex
}

// NewTokenBucket 创建令牌桶
func NewTokenBucket(capacity int, refillWindow time.Duration) *TokenBucket {
	return &TokenBucket{
		capacity:   capacity,
		refillRate: refillWindow / time.Duration(capacity),
		tokens:     capacity,
		lastRefill: time.Now(),
		lastAccess: time.Now(),
	}
}

// Allow 检查是否允许请求
func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	tb.lastAccess = now

	// 补充令牌
	elapsed := now.Sub(tb.lastRefill)
	tokensToAdd := int(elapsed / tb.refillRate)
	if tokensToAdd > 0 {
		tb.tokens = min(tb.capacity, tb.tokens+tokensToAdd)
		tb.lastRefill = now
	}

	// 检查是否有可用令牌
	if tb.tokens > 0 {
		tb.tokens--
		return true
	}

	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

