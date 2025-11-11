package imapd

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"

	"github.com/emersion/go-imap/server"
	"github.com/gomailzero/gmz/internal/logger"
	"github.com/gomailzero/gmz/internal/storage"
)

// Server IMAP 服务器
type Server struct {
	config  *Config
	backend *Backend
	server  *server.Server
}

// Config IMAP 配置
type Config struct {
	Enabled bool
	Port    int
	TLS     *tls.Config
	Storage storage.Driver
	Auth    Authenticator
}

// NewServer 创建 IMAP 服务器
func NewServer(cfg *Config) *Server {
	backend := NewBackend(cfg.Storage, cfg.Auth)

	s := server.New(backend)
	s.Addr = fmt.Sprintf(":%d", cfg.Port)
	s.AllowInsecureAuth = false // 强制 TLS

	if cfg.TLS != nil {
		s.TLSConfig = cfg.TLS
	}

	return &Server{
		config:  cfg,
		backend: backend,
		server:  s,
	}
}

// Start 启动服务器
func (s *Server) Start(ctx context.Context) error {
	if !s.config.Enabled {
		logger.Info().Msg("IMAP 服务器已禁用")
		return nil
	}

	listener, err := net.Listen("tcp", s.server.Addr)
	if err != nil {
		return fmt.Errorf("监听端口失败: %w", err)
	}

	// 使用 TLS
	if s.config.TLS != nil {
		listener = tls.NewListener(listener, s.config.TLS)
	}

	logger.Info().Int("port", s.config.Port).Msg("IMAP 服务器启动")

	if err := s.server.Serve(listener); err != nil {
		return fmt.Errorf("IMAP 服务器错误: %w", err)
	}

	return nil
}

// Stop 停止服务器
func (s *Server) Stop(ctx context.Context) error {
	if err := s.server.Close(); err != nil {
		logger.Error().Err(err).Msg("关闭 IMAP 服务器失败")
		return err
	}

	logger.Info().Msg("IMAP 服务器已停止")
	return nil
}

