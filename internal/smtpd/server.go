package smtpd

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"sync"

	"github.com/emersion/go-smtp"
	"github.com/gomailzero/gmz/internal/logger"
	"github.com/gomailzero/gmz/internal/storage"
)

// Server SMTP 服务器
type Server struct {
	config  *Config
	backend *Backend
	servers []*smtp.Server
	wg      sync.WaitGroup
}

// Config SMTP 配置
type Config struct {
	Enabled  bool
	Ports    []int
	Hostname string
	MaxSize  int64
	TLS      *tls.Config
	Storage  storage.Driver
	Maildir  *storage.Maildir
	Auth     Authenticator
}

// NewServer 创建 SMTP 服务器
func NewServer(cfg *Config) *Server {
	backend := NewBackend(cfg.Storage, cfg.Maildir, cfg.Auth)

	s := smtp.NewServer(backend)
	s.Addr = fmt.Sprintf(":%d", cfg.Ports[0])
	s.Domain = cfg.Hostname
	if s.Domain == "" {
		s.Domain = "localhost"
	}
	s.MaxMessageBytes = int64(cfg.MaxSize)
	s.MaxRecipients = 100

	if cfg.TLS != nil {
		s.TLSConfig = cfg.TLS
		// TODO: 实现认证支持
	}

	return &Server{
		config:  cfg,
		backend: backend,
		servers: []*smtp.Server{s},
	}
}

// Start 启动服务器
func (s *Server) Start(ctx context.Context) error {
	if !s.config.Enabled {
		logger.Info().Msg("SMTP 服务器已禁用")
		return nil
	}

	for _, port := range s.config.Ports {
		s.wg.Add(1)
		go func(p int) {
			defer s.wg.Done()

			addr := fmt.Sprintf(":%d", p)
			listener, err := net.Listen("tcp", addr)
			if err != nil {
				logger.Error().Err(err).Int("port", p).Msg("监听端口失败")
				return
			}

			// 如果是 465 端口，使用 TLS
			if p == 465 && s.config.TLS != nil {
				listener = tls.NewListener(listener, s.config.TLS)
			}

			logger.Info().Int("port", p).Msg("SMTP 服务器启动")

			if err := s.servers[0].Serve(listener); err != nil {
				logger.Error().Err(err).Int("port", p).Msg("SMTP 服务器错误")
			}
		}(port)
	}

	return nil
}

// Stop 停止服务器
func (s *Server) Stop(ctx context.Context) error {
	for _, server := range s.servers {
		if err := server.Close(); err != nil {
			logger.Error().Err(err).Msg("关闭 SMTP 服务器失败")
		}
	}

	s.wg.Wait()
	logger.Info().Msg("SMTP 服务器已停止")
	return nil
}

