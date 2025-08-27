package main

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// AdvancedAnalytics 高级分析系统
type AdvancedAnalytics struct {
	mu                sync.RWMutex
	requestPatterns   map[string]*AnalyticsRequestPattern
	userBehavior      map[string]*UserBehavior
	costAnalysis      *CostAnalysis
	performanceMetrics *PerformanceMetrics
	startTime         time.Time
}

// AnalyticsRequestPattern 分析请求模式
type AnalyticsRequestPattern struct {
	Pattern     string    `json:"pattern"`
	Frequency   int64     `json:"frequency"`
	AvgSize     int       `json:"avg_size"`
	AvgDuration time.Duration `json:"avg_duration"`
	LastSeen    time.Time `json:"last_seen"`
	Trend       string    `json:"trend"` // increasing, decreasing, stable
}

// UserBehavior 用户行为分析
type UserBehavior struct {
	UserID          string    `json:"user_id"`
	RequestCount    int64     `json:"request_count"`
	AvgRequestSize  int       `json:"avg_request_size"`
	PeakHours       []int     `json:"peak_hours"`
	PreferredModels []string  `json:"preferred_models"`
	LastActive      time.Time `json:"last_active"`
}

// CostAnalysis 成本分析
type CostAnalysis struct {
	TotalRequests      int64   `json:"total_requests"`
	CachedRequests     int64   `json:"cached_requests"`
	EstimatedCostSaved float64 `json:"estimated_cost_saved"`
	CostPerRequest     float64 `json:"cost_per_request"`
	MonthlySavings     float64 `json:"monthly_savings"`
}

// PerformanceMetrics 性能指标
type PerformanceMetrics struct {
	AvgResponseTime    time.Duration `json:"avg_response_time"`
	P95ResponseTime    time.Duration `json:"p95_response_time"`
	P99ResponseTime    time.Duration `json:"p99_response_time"`
	ThroughputPerSec   float64       `json:"throughput_per_sec"`
	ErrorRate          float64       `json:"error_rate"`
	CacheHitRate       float64       `json:"cache_hit_rate"`
	ResponseTimes      []time.Duration `json:"-"` // 不导出，用于计算百分位数
}

var advancedAnalytics = &AdvancedAnalytics{
	requestPatterns:    make(map[string]*AnalyticsRequestPattern),
	userBehavior:       make(map[string]*UserBehavior),
	costAnalysis:       &CostAnalysis{CostPerRequest: 0.001}, // 假设每个请求成本0.001美元
	performanceMetrics: &PerformanceMetrics{ResponseTimes: make([]time.Duration, 0, 10000)},
	startTime:          time.Now(),
}

// RecordRequest 记录请求用于分析
func (aa *AdvancedAnalytics) RecordRequest(req AnthropicRequest, userID string, responseTime time.Duration, cached bool, size int) {
	aa.mu.Lock()
	defer aa.mu.Unlock()

	// 分析请求模式
	pattern := aa.generatePattern(req)
	if p, exists := aa.requestPatterns[pattern]; exists {
		p.Frequency++
		p.AvgSize = (p.AvgSize + size) / 2
		p.AvgDuration = (p.AvgDuration + responseTime) / 2
		p.LastSeen = time.Now()
		p.Trend = aa.calculateTrend(p)
	} else {
		aa.requestPatterns[pattern] = &AnalyticsRequestPattern{
			Pattern:     pattern,
			Frequency:   1,
			AvgSize:     size,
			AvgDuration: responseTime,
			LastSeen:    time.Now(),
			Trend:       "new",
		}
	}

	// 分析用户行为
	if userID != "" {
		if ub, exists := aa.userBehavior[userID]; exists {
			ub.RequestCount++
			ub.AvgRequestSize = (ub.AvgRequestSize + size) / 2
			ub.LastActive = time.Now()
			aa.updatePeakHours(ub)
			aa.updatePreferredModels(ub, req.Model)
		} else {
			aa.userBehavior[userID] = &UserBehavior{
				UserID:          userID,
				RequestCount:    1,
				AvgRequestSize:  size,
				PeakHours:       []int{time.Now().Hour()},
				PreferredModels: []string{req.Model},
				LastActive:      time.Now(),
			}
		}
	}

	// 更新成本分析
	aa.costAnalysis.TotalRequests++
	if cached {
		aa.costAnalysis.CachedRequests++
	}
	aa.updateCostAnalysis()

	// 更新性能指标
	aa.updatePerformanceMetrics(responseTime, cached)
}

// generatePattern 生成请求模式
func (aa *AdvancedAnalytics) generatePattern(req AnthropicRequest) string {
	return fmt.Sprintf("%s_%d_msgs", req.Model, len(req.Messages))
}

// calculateTrend 计算趋势
func (aa *AdvancedAnalytics) calculateTrend(p *AnalyticsRequestPattern) string {
	// 简化的趋势计算
	if p.Frequency > 10 {
		return "increasing"
	} else if p.Frequency < 5 {
		return "decreasing"
	}
	return "stable"
}

// updatePeakHours 更新高峰时段
func (aa *AdvancedAnalytics) updatePeakHours(ub *UserBehavior) {
	currentHour := time.Now().Hour()
	found := false
	for _, hour := range ub.PeakHours {
		if hour == currentHour {
			found = true
			break
		}
	}
	if !found {
		ub.PeakHours = append(ub.PeakHours, currentHour)
		// 保持最多5个高峰时段
		if len(ub.PeakHours) > 5 {
			ub.PeakHours = ub.PeakHours[1:]
		}
	}
}

// updatePreferredModels 更新偏好模型
func (aa *AdvancedAnalytics) updatePreferredModels(ub *UserBehavior, model string) {
	found := false
	for _, m := range ub.PreferredModels {
		if m == model {
			found = true
			break
		}
	}
	if !found {
		ub.PreferredModels = append(ub.PreferredModels, model)
		// 保持最多3个偏好模型
		if len(ub.PreferredModels) > 3 {
			ub.PreferredModels = ub.PreferredModels[1:]
		}
	}
}

// updateCostAnalysis 更新成本分析
func (aa *AdvancedAnalytics) updateCostAnalysis() {
	if aa.costAnalysis.TotalRequests > 0 {
		savedRequests := aa.costAnalysis.CachedRequests
		aa.costAnalysis.EstimatedCostSaved = float64(savedRequests) * aa.costAnalysis.CostPerRequest
		
		// 计算月度节省（假设当前速率持续一个月）
		uptime := time.Since(aa.startTime)
		if uptime > 0 {
			requestsPerMonth := float64(aa.costAnalysis.TotalRequests) * (30 * 24 * time.Hour).Seconds() / uptime.Seconds()
			cacheRate := float64(aa.costAnalysis.CachedRequests) / float64(aa.costAnalysis.TotalRequests)
			aa.costAnalysis.MonthlySavings = requestsPerMonth * cacheRate * aa.costAnalysis.CostPerRequest
		}
	}
}

// updatePerformanceMetrics 更新性能指标
func (aa *AdvancedAnalytics) updatePerformanceMetrics(responseTime time.Duration, cached bool) {
	pm := aa.performanceMetrics
	
	// 添加响应时间到列表
	pm.ResponseTimes = append(pm.ResponseTimes, responseTime)
	
	// 保持最多10000个样本
	if len(pm.ResponseTimes) > 10000 {
		pm.ResponseTimes = pm.ResponseTimes[1:]
	}
	
	// 计算平均响应时间
	var total time.Duration
	for _, rt := range pm.ResponseTimes {
		total += rt
	}
	pm.AvgResponseTime = total / time.Duration(len(pm.ResponseTimes))
	
	// 计算百分位数
	if len(pm.ResponseTimes) > 10 {
		sorted := make([]time.Duration, len(pm.ResponseTimes))
		copy(sorted, pm.ResponseTimes)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i] < sorted[j]
		})
		
		p95Index := int(math.Ceil(0.95 * float64(len(sorted)))) - 1
		p99Index := int(math.Ceil(0.99 * float64(len(sorted)))) - 1
		
		if p95Index >= 0 && p95Index < len(sorted) {
			pm.P95ResponseTime = sorted[p95Index]
		}
		if p99Index >= 0 && p99Index < len(sorted) {
			pm.P99ResponseTime = sorted[p99Index]
		}
	}
	
	// 计算吞吐量
	uptime := time.Since(aa.startTime)
	if uptime > 0 {
		pm.ThroughputPerSec = float64(len(pm.ResponseTimes)) / uptime.Seconds()
	}
	
	// 更新缓存命中率
	if aa.costAnalysis.TotalRequests > 0 {
		pm.CacheHitRate = float64(aa.costAnalysis.CachedRequests) / float64(aa.costAnalysis.TotalRequests) * 100
	}
}

// GetAnalytics 获取分析报告
func (aa *AdvancedAnalytics) GetAnalytics() map[string]interface{} {
	aa.mu.RLock()
	defer aa.mu.RUnlock()

	// 获取热门请求模式
	var topPatterns []*AnalyticsRequestPattern
	for _, pattern := range aa.requestPatterns {
		topPatterns = append(topPatterns, pattern)
	}
	sort.Slice(topPatterns, func(i, j int) bool {
		return topPatterns[i].Frequency > topPatterns[j].Frequency
	})
	if len(topPatterns) > 10 {
		topPatterns = topPatterns[:10]
	}

	// 获取活跃用户
	var activeUsers []*UserBehavior
	for _, user := range aa.userBehavior {
		if time.Since(user.LastActive) < 24*time.Hour {
			activeUsers = append(activeUsers, user)
		}
	}

	return map[string]interface{}{
		"request_patterns":    topPatterns,
		"active_users":        len(activeUsers),
		"total_users":         len(aa.userBehavior),
		"cost_analysis":       aa.costAnalysis,
		"performance_metrics": aa.performanceMetrics,
		"uptime_hours":        time.Since(aa.startTime).Hours(),
		"analysis_timestamp":  time.Now().Unix(),
	}
}

// GetRecommendations 获取优化建议
func (aa *AdvancedAnalytics) GetRecommendations() []string {
	aa.mu.RLock()
	defer aa.mu.RUnlock()

	var recommendations []string

	// 基于缓存命中率的建议
	if aa.performanceMetrics.CacheHitRate < 30 {
		recommendations = append(recommendations, "缓存命中率较低，建议增加缓存大小或调整缓存策略")
	}

	// 基于响应时间的建议
	if aa.performanceMetrics.AvgResponseTime > 2*time.Second {
		recommendations = append(recommendations, "平均响应时间较长，建议启用更多优化功能")
	}

	// 基于请求模式的建议
	for _, pattern := range aa.requestPatterns {
		if pattern.Frequency > 100 && pattern.Trend == "increasing" {
			recommendations = append(recommendations, fmt.Sprintf("检测到高频请求模式 %s，建议为此模式设置专门的缓存策略", pattern.Pattern))
		}
	}

	// 基于成本的建议
	if aa.costAnalysis.MonthlySavings > 100 {
		recommendations = append(recommendations, fmt.Sprintf("当前优化每月可节省约 $%.2f，建议继续保持当前配置", aa.costAnalysis.MonthlySavings))
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "系统运行良好，当前优化配置表现出色")
	}

	return recommendations
}
