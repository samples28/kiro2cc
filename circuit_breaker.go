package main

import (
	"errors"
	"sync"
	"time"
)

// CircuitBreakerState 熔断器状态
type CircuitBreakerState int

const (
	StateClosed CircuitBreakerState = iota
	StateOpen
	StateHalfOpen
)

// CircuitBreaker 熔断器
type CircuitBreaker struct {
	mu                sync.RWMutex
	state             CircuitBreakerState
	failureCount      int64
	successCount      int64
	lastFailureTime   time.Time
	lastSuccessTime   time.Time
	
	// 配置参数
	maxFailures       int64         // 最大失败次数
	timeout           time.Duration // 熔断超时时间
	halfOpenMaxCalls  int64         // 半开状态最大调用次数
	halfOpenSuccessThreshold int64  // 半开状态成功阈值
	
	// 统计信息
	totalCalls        int64
	totalFailures     int64
	totalSuccesses    int64
	stateChanges      int64
	lastStateChange   time.Time
}

var circuitBreaker = &CircuitBreaker{
	state:                    StateClosed,
	maxFailures:              5,
	timeout:                  30 * time.Second,
	halfOpenMaxCalls:         3,
	halfOpenSuccessThreshold: 2,
	lastStateChange:          time.Now(),
}

// ErrCircuitBreakerOpen 熔断器开启错误
var ErrCircuitBreakerOpen = errors.New("circuit breaker is open")

// Call 执行调用
func (cb *CircuitBreaker) Call(fn func() error) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.totalCalls++

	// 检查当前状态
	switch cb.state {
	case StateClosed:
		return cb.callInClosedState(fn)
	case StateOpen:
		return cb.callInOpenState(fn)
	case StateHalfOpen:
		return cb.callInHalfOpenState(fn)
	default:
		return cb.callInClosedState(fn)
	}
}

// callInClosedState 在关闭状态下执行调用
func (cb *CircuitBreaker) callInClosedState(fn func() error) error {
	err := fn()
	
	if err != nil {
		cb.onFailure()
		if cb.failureCount >= cb.maxFailures {
			cb.setState(StateOpen)
		}
		return err
	}
	
	cb.onSuccess()
	return nil
}

// callInOpenState 在开启状态下执行调用
func (cb *CircuitBreaker) callInOpenState(fn func() error) error {
	// 检查是否可以转换到半开状态
	if time.Since(cb.lastFailureTime) >= cb.timeout {
		cb.setState(StateHalfOpen)
		return cb.callInHalfOpenState(fn)
	}
	
	return ErrCircuitBreakerOpen
}

// callInHalfOpenState 在半开状态下执行调用
func (cb *CircuitBreaker) callInHalfOpenState(fn func() error) error {
	// 限制半开状态下的调用次数
	if cb.successCount+cb.failureCount >= cb.halfOpenMaxCalls {
		if cb.successCount >= cb.halfOpenSuccessThreshold {
			cb.setState(StateClosed)
		} else {
			cb.setState(StateOpen)
		}
		// 重置计数器
		cb.successCount = 0
		cb.failureCount = 0
	}
	
	err := fn()
	
	if err != nil {
		cb.onFailure()
		cb.setState(StateOpen)
		return err
	}
	
	cb.onSuccess()
	
	// 检查是否可以关闭熔断器
	if cb.successCount >= cb.halfOpenSuccessThreshold {
		cb.setState(StateClosed)
	}
	
	return nil
}

// onSuccess 成功回调
func (cb *CircuitBreaker) onSuccess() {
	cb.successCount++
	cb.totalSuccesses++
	cb.lastSuccessTime = time.Now()
	
	// 在关闭状态下，成功调用会重置失败计数
	if cb.state == StateClosed {
		cb.failureCount = 0
	}
}

// onFailure 失败回调
func (cb *CircuitBreaker) onFailure() {
	cb.failureCount++
	cb.totalFailures++
	cb.lastFailureTime = time.Now()
}

// setState 设置状态
func (cb *CircuitBreaker) setState(state CircuitBreakerState) {
	if cb.state != state {
		cb.state = state
		cb.stateChanges++
		cb.lastStateChange = time.Now()
		
		// 状态变化时重置相关计数器
		if state == StateClosed {
			cb.failureCount = 0
			cb.successCount = 0
		} else if state == StateHalfOpen {
			cb.successCount = 0
			cb.failureCount = 0
		}
	}
}

// GetState 获取当前状态
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetStats 获取统计信息
func (cb *CircuitBreaker) GetStats() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	var stateStr string
	switch cb.state {
	case StateClosed:
		stateStr = "closed"
	case StateOpen:
		stateStr = "open"
	case StateHalfOpen:
		stateStr = "half-open"
	}

	successRate := 0.0
	if cb.totalCalls > 0 {
		successRate = float64(cb.totalSuccesses) / float64(cb.totalCalls) * 100
	}

	return map[string]interface{}{
		"state":                    stateStr,
		"total_calls":              cb.totalCalls,
		"total_successes":          cb.totalSuccesses,
		"total_failures":           cb.totalFailures,
		"success_rate":             successRate,
		"current_failure_count":    cb.failureCount,
		"current_success_count":    cb.successCount,
		"state_changes":            cb.stateChanges,
		"last_state_change":        cb.lastStateChange.Unix(),
		"last_failure_time":        cb.lastFailureTime.Unix(),
		"last_success_time":        cb.lastSuccessTime.Unix(),
		"max_failures":             cb.maxFailures,
		"timeout_seconds":          cb.timeout.Seconds(),
		"half_open_max_calls":      cb.halfOpenMaxCalls,
		"half_open_success_threshold": cb.halfOpenSuccessThreshold,
	}
}

// Reset 重置熔断器
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = StateClosed
	cb.failureCount = 0
	cb.successCount = 0
	cb.totalCalls = 0
	cb.totalFailures = 0
	cb.totalSuccesses = 0
	cb.stateChanges = 0
	cb.lastStateChange = time.Now()
}

// Configure 配置熔断器参数
func (cb *CircuitBreaker) Configure(maxFailures int64, timeout time.Duration, halfOpenMaxCalls, halfOpenSuccessThreshold int64) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.maxFailures = maxFailures
	cb.timeout = timeout
	cb.halfOpenMaxCalls = halfOpenMaxCalls
	cb.halfOpenSuccessThreshold = halfOpenSuccessThreshold
}

// IsCallAllowed 检查是否允许调用
func (cb *CircuitBreaker) IsCallAllowed() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		return time.Since(cb.lastFailureTime) >= cb.timeout
	case StateHalfOpen:
		return cb.successCount+cb.failureCount < cb.halfOpenMaxCalls
	default:
		return true
	}
}

// GetHealthStatus 获取健康状态
func (cb *CircuitBreaker) GetHealthStatus() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	var health string
	var recommendation string

	switch cb.state {
	case StateClosed:
		if cb.failureCount == 0 {
			health = "excellent"
			recommendation = "系统运行正常"
		} else if cb.failureCount < cb.maxFailures/2 {
			health = "good"
			recommendation = "系统运行良好，有少量错误"
		} else {
			health = "warning"
			recommendation = "错误率较高，需要关注"
		}
	case StateOpen:
		health = "critical"
		recommendation = "熔断器已开启，系统暂时不可用"
	case StateHalfOpen:
		health = "recovering"
		recommendation = "系统正在恢复中，请监控"
	}

	return map[string]interface{}{
		"health":         health,
		"recommendation": recommendation,
		"uptime_seconds": time.Since(cb.lastStateChange).Seconds(),
		"is_available":   cb.IsCallAllowed(),
	}
}

// CircuitBreakerMiddleware 熔断器中间件
func CircuitBreakerMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := circuitBreaker.Call(func() error {
			// 创建一个响应写入器来捕获状态码
			rw := &responseWriter{ResponseWriter: w, statusCode: 200}
			next(rw, r)
			
			// 如果状态码表示错误，返回错误
			if rw.statusCode >= 500 {
				return errors.New("server error")
			}
			return nil
		})

		if err != nil {
			if err == ErrCircuitBreakerOpen {
				w.Header().Set("X-Circuit-Breaker", "open")
				http.Error(w, "Service temporarily unavailable due to circuit breaker", http.StatusServiceUnavailable)
			}
			// 其他错误已经在next函数中处理
		}
	}
}
