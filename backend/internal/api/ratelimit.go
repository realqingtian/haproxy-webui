package api

import (
	"sync"
	"time"
)

// loginLimiter 是登录失败的滑动窗口限流器(按 IP + 用户名)。
// 规则:窗口期内失败达到 maxFailures 次后,锁定 lockDuration;登录成功即清零。
// 仅存内存,重启即清零——目的是提高暴力破解成本,不是强风控。
type loginLimiter struct {
	mu          sync.Mutex
	failures    map[string][]time.Time
	window      time.Duration
	maxFailures int
	lock        time.Duration
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{
		failures:    make(map[string][]time.Time),
		window:      time.Minute,
		maxFailures: 5,
		lock:        time.Minute,
	}
}

// Blocked 返回该 key 当前是否被锁定。
func (l *loginLimiter) Blocked(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	ts := l.recentFailuresLocked(key)
	return len(ts) >= l.maxFailures && time.Since(ts[len(ts)-1]) < l.lock
}

// Fail 记录一次失败。
func (l *loginLimiter) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.failures[key] = append(l.recentFailuresLocked(key), time.Now())
}

// Reset 登录成功后清零。
func (l *loginLimiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.failures, key)
}

func (l *loginLimiter) recentFailuresLocked(key string) []time.Time {
	all := l.failures[key]
	recent := all[:0]
	cutoff := time.Now().Add(-l.window)
	for _, t := range all {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	if len(recent) == 0 {
		delete(l.failures, key)
		return nil
	}
	l.failures[key] = recent
	return recent
}
