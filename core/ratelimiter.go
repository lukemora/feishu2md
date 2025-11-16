package core

import (
	"context"
	"time"

	"golang.org/x/time/rate"
)

// FeishuRateLimiter 飞书API限流器
// 限制: 100次/分钟 且 5次/秒
type FeishuRateLimiter struct {
	perSecond *rate.Limiter // 5次/秒限制
	perMinute *rate.Limiter // 100次/分钟限制
}

// NewFeishuRateLimiter 创建飞书API限流器
func NewFeishuRateLimiter() *FeishuRateLimiter {
	return &FeishuRateLimiter{
		// 5次/秒，burst设为5允许短时突发
		perSecond: rate.NewLimiter(rate.Limit(5), 5),
		
		// 100次/分钟 = 1.67次/秒，burst设为10允许初始突发
		perMinute: rate.NewLimiter(rate.Every(time.Minute/100), 10),
	}
}

// Wait 等待直到可以执行飞书API请求
// 必须同时满足两个限流器的条件
func (l *FeishuRateLimiter) Wait(ctx context.Context) error {
	// 先检查秒级限流
	if err := l.perSecond.Wait(ctx); err != nil {
		return err
	}
	
	// 再检查分钟级限流
	return l.perMinute.Wait(ctx)
}

// WaitN 等待N个令牌
func (l *FeishuRateLimiter) WaitN(ctx context.Context, n int) error {
	if err := l.perSecond.WaitN(ctx, n); err != nil {
		return err
	}
	return l.perMinute.WaitN(ctx, n)
}

// Allow 检查是否可以立即执行（不等待）
func (l *FeishuRateLimiter) Allow() bool {
	return l.perSecond.Allow() && l.perMinute.Allow()
}

// AllowN 检查是否可以立即执行N次
func (l *FeishuRateLimiter) AllowN(n int) bool {
	return l.perSecond.AllowN(time.Now(), n) && l.perMinute.AllowN(time.Now(), n)
}

