package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Exporter Prometheus 指标导出器
type Exporter struct {
	registry *prometheus.Registry

	// SMTP 指标
	smtpConnections  prometheus.Gauge
	smtpMessages     prometheus.Counter
	smtpErrors       prometheus.Counter
	smtpAuthFailures prometheus.Counter

	// IMAP 指标
	imapConnections  prometheus.Gauge
	imapOperations   prometheus.Counter
	imapErrors       prometheus.Counter
	imapAuthFailures prometheus.Counter

	// 队列指标
	queueSize      prometheus.Gauge
	queueProcessed prometheus.Counter

	// TLS 指标
	tlsHandshakes      prometheus.Counter
	tlsHandshakeErrors prometheus.Counter
	tlsCertExpiry      prometheus.Gauge

	// 存储指标
	storageSize prometheus.Gauge
	mailCount   prometheus.Gauge
}

// NewExporter 创建指标导出器
func NewExporter() *Exporter {
	registry := prometheus.NewRegistry()

	exporter := &Exporter{
		registry: registry,

		// SMTP 指标
		smtpConnections: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gmz_smtp_connections",
			Help: "当前 SMTP 连接数",
		}),
		smtpMessages: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "gmz_smtp_messages_total",
			Help: "SMTP 消息总数",
		}),
		smtpErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "gmz_smtp_errors_total",
			Help: "SMTP 错误总数",
		}),
		smtpAuthFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "gmz_smtp_auth_failures_total",
			Help: "SMTP 认证失败总数",
		}),

		// IMAP 指标
		imapConnections: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gmz_imap_connections",
			Help: "当前 IMAP 连接数",
		}),
		imapOperations: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "gmz_imap_operations_total",
			Help: "IMAP 操作总数",
		}),
		imapErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "gmz_imap_errors_total",
			Help: "IMAP 错误总数",
		}),
		imapAuthFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "gmz_imap_auth_failures_total",
			Help: "IMAP 认证失败总数",
		}),

		// 队列指标
		queueSize: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gmz_queue_size",
			Help: "队列大小",
		}),
		queueProcessed: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "gmz_queue_processed_total",
			Help: "已处理的队列项总数",
		}),

		// TLS 指标
		tlsHandshakes: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "gmz_tls_handshakes_total",
			Help: "TLS 握手总数",
		}),
		tlsHandshakeErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "gmz_tls_handshake_errors_total",
			Help: "TLS 握手错误总数",
		}),
		tlsCertExpiry: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gmz_tls_cert_expiry_seconds",
			Help: "TLS 证书过期时间（秒）",
		}),

		// 存储指标
		storageSize: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gmz_storage_size_bytes",
			Help: "存储大小（字节）",
		}),
		mailCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gmz_mail_count",
			Help: "邮件总数",
		}),
	}

	// 注册指标
	registry.MustRegister(
		exporter.smtpConnections,
		exporter.smtpMessages,
		exporter.smtpErrors,
		exporter.smtpAuthFailures,
		exporter.imapConnections,
		exporter.imapOperations,
		exporter.imapErrors,
		exporter.imapAuthFailures,
		exporter.queueSize,
		exporter.queueProcessed,
		exporter.tlsHandshakes,
		exporter.tlsHandshakeErrors,
		exporter.tlsCertExpiry,
		exporter.storageSize,
		exporter.mailCount,
	)

	return exporter
}

// Handler 返回 HTTP 处理器
func (e *Exporter) Handler() http.Handler {
	return promhttp.HandlerFor(e.registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})
}

// IncSMTPConnections 增加 SMTP 连接数
func (e *Exporter) IncSMTPConnections() {
	e.smtpConnections.Inc()
}

// DecSMTPConnections 减少 SMTP 连接数
func (e *Exporter) DecSMTPConnections() {
	e.smtpConnections.Dec()
}

// IncSMTPMessages 增加 SMTP 消息数
func (e *Exporter) IncSMTPMessages() {
	e.smtpMessages.Inc()
}

// IncSMTPErrors 增加 SMTP 错误数
func (e *Exporter) IncSMTPErrors() {
	e.smtpErrors.Inc()
}

// IncSMTPAuthFailures 增加 SMTP 认证失败数
func (e *Exporter) IncSMTPAuthFailures() {
	e.smtpAuthFailures.Inc()
}

// IncIMAPConnections 增加 IMAP 连接数
func (e *Exporter) IncIMAPConnections() {
	e.imapConnections.Inc()
}

// DecIMAPConnections 减少 IMAP 连接数
func (e *Exporter) DecIMAPConnections() {
	e.imapConnections.Dec()
}

// IncIMAPOperations 增加 IMAP 操作数
func (e *Exporter) IncIMAPOperations() {
	e.imapOperations.Inc()
}

// IncIMAPErrors 增加 IMAP 错误数
func (e *Exporter) IncIMAPErrors() {
	e.imapErrors.Inc()
}

// IncIMAPAuthFailures 增加 IMAP 认证失败数
func (e *Exporter) IncIMAPAuthFailures() {
	e.imapAuthFailures.Inc()
}

// SetQueueSize 设置队列大小
func (e *Exporter) SetQueueSize(size float64) {
	e.queueSize.Set(size)
}

// IncQueueProcessed 增加已处理的队列项数
func (e *Exporter) IncQueueProcessed() {
	e.queueProcessed.Inc()
}

// IncTLSHandshakes 增加 TLS 握手数
func (e *Exporter) IncTLSHandshakes() {
	e.tlsHandshakes.Inc()
}

// IncTLSHandshakeErrors 增加 TLS 握手错误数
func (e *Exporter) IncTLSHandshakeErrors() {
	e.tlsHandshakeErrors.Inc()
}

// SetTLSCertExpiry 设置 TLS 证书过期时间
func (e *Exporter) SetTLSCertExpiry(expiry time.Time) {
	e.tlsCertExpiry.Set(float64(expiry.Unix()))
}

// SetStorageSize 设置存储大小
func (e *Exporter) SetStorageSize(size float64) {
	e.storageSize.Set(size)
}

// SetMailCount 设置邮件总数
func (e *Exporter) SetMailCount(count float64) {
	e.mailCount.Set(count)
}
