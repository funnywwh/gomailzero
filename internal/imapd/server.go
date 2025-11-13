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
	Maildir *storage.Maildir // Maildir 实例，用于读取邮件体
	Auth    Authenticator
}

// NewServer 创建 IMAP 服务器
func NewServer(cfg *Config) *Server {
	bkd := NewBackend(cfg.Storage, cfg.Maildir, cfg.Auth)

	s := server.New(bkd)
	s.Addr = fmt.Sprintf(":%d", cfg.Port)
	
	// 如果配置了 TLS，强制使用 TLS；否则允许非安全连接（仅用于开发环境）
	if cfg.TLS != nil {
		s.AllowInsecureAuth = false // 强制 TLS
		s.TLSConfig = cfg.TLS
	} else {
		// 警告：生产环境不应该允许非安全连接
		logger.Warn().Msg("IMAP 服务器未配置 TLS，允许非安全连接（仅用于开发环境）")
		s.AllowInsecureAuth = true
	}

	return &Server{
		config:  cfg,
		backend: bkd,
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

	// 使用 TLS（如果已配置）
	if s.config.TLS != nil {
		if len(s.config.TLS.Certificates) == 0 {
			return fmt.Errorf("IMAP 服务器 TLS 已启用但未配置证书，请检查 TLS 配置")
		}
		listener = tls.NewListener(listener, s.config.TLS)
		logger.Info().Msg("IMAP 服务器使用 TLS")
	} else {
		logger.Warn().Msg("IMAP 服务器未使用 TLS（仅用于开发环境）")
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
