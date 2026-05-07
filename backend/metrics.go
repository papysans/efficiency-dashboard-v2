package main

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	metricsNamespace = "efficiency_dashboard"
)

var (
	// httpRequestsTotal 统计 HTTP 请求总数，按方法、路径、状态码分类
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests, labeled by method, path and status code",
		},
		[]string{"method", "path", "status"},
	)

	// httpRequestDurationSeconds 统计 HTTP 请求响应时延
	httpRequestDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: metricsNamespace,
			Name:      "http_request_duration_seconds",
			Help:      "Duration of HTTP requests in seconds, labeled by method and path",
			Buckets:   prometheus.DefBuckets, // {.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}
		},
		[]string{"method", "path"},
	)
)

// metricsResponseWriter 包装 gin.ResponseWriter 以捕获状态码
type metricsResponseWriter struct {
	gin.ResponseWriter
	statusCode int
}

func newMetricsResponseWriter(w gin.ResponseWriter) *metricsResponseWriter {
	return &metricsResponseWriter{
		ResponseWriter: w,
		statusCode:     200, // gin 默认状态码为 200
	}
}

func (w *metricsResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

// MetricsMiddleware 返回一个 Gin 中间件，用于收集请求指标
// 该中间件无侵入地统计所有 API 的响应码和响应时延
func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 跳过 /metrics 端点自身的请求，避免递归统计
		if c.Request.URL.Path == "/metrics" {
			c.Next()
			return
		}

		start := time.Now()

		// 包装 ResponseWriter 以捕获状态码
		wrappedWriter := newMetricsResponseWriter(c.Writer)
		c.Writer = wrappedWriter

		// 执行后续处理
		c.Next()

		// 记录指标
		elapsed := time.Since(start).Seconds()
		status := strconv.Itoa(wrappedWriter.statusCode)
		path := c.FullPath() // 使用路由模板而非实际路径，避免高基数
		if path == "" {
			path = c.Request.URL.Path // 兜底：未匹配路由时使用实际路径
		}
		method := c.Request.Method

		httpRequestsTotal.WithLabelValues(method, path, status).Inc()
		httpRequestDurationSeconds.WithLabelValues(method, path).Observe(elapsed)
	}
}

// MetricsHandler 返回 Prometheus 标准的 /metrics 端点处理器
func MetricsHandler() gin.HandlerFunc {
	h := promhttp.Handler()
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}
