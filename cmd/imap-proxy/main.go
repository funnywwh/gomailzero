package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	listenAddr     = flag.String("listen", ":1993", "监听地址（客户端连接地址）")
	targetAddr     = flag.String("target", "localhost:993", "目标 IMAP 服务器地址")
	useTLS         = flag.Bool("tls", true, "是否使用 TLS 连接目标服务器")
	clientTLS      = flag.Bool("client-tls", false, "是否接受客户端的 TLS 连接（TLS-in-TLS 模式）")
	clientCertFile = flag.String("client-cert", "", "客户端 TLS 证书文件（用于 -client-tls）")
	clientKeyFile  = flag.String("client-key", "", "客户端 TLS 密钥文件（用于 -client-tls）")
	insecureTLS    = flag.Bool("insecure", false, "跳过 TLS 证书验证（仅用于调试）")
	logFile        = flag.String("log", "", "日志文件路径（留空自动生成：logs/imap-proxy-YYYYMMDD-HHMMSS.log）")
	logDir         = flag.String("log-dir", "logs", "日志目录（自动创建）")
	autoLog        = flag.Bool("auto-log", true, "自动保存日志到文件（默认启用）")
	verbose        = flag.Bool("v", false, "详细输出模式")
)

// Proxy 透传代理
type Proxy struct {
	listenAddr      string
	targetAddr      string
	useTLS          bool
	clientTLS       bool
	clientTLSConfig *tls.Config
	insecureTLS     bool
	logFile         *os.File
	logger          *log.Logger
	verbose         bool
}

// NewProxy 创建新的代理实例
func NewProxy() (*Proxy, error) {
	p := &Proxy{
		listenAddr:  *listenAddr,
		targetAddr:  *targetAddr,
		useTLS:      *useTLS,
		clientTLS:   *clientTLS,
		insecureTLS: *insecureTLS,
		verbose:     *verbose,
	}

	// 如果启用客户端 TLS，加载证书
	if p.clientTLS {
		if *clientCertFile == "" || *clientKeyFile == "" {
			// 尝试生成自签名证书
			cert, err := generateSelfSignedCert()
			if err != nil {
				return nil, fmt.Errorf("生成自签名证书失败: %w", err)
			}
			p.clientTLSConfig = &tls.Config{
				Certificates: []tls.Certificate{cert},
			}
			p.logger = log.New(os.Stderr, "", log.LstdFlags)
			p.logger.Printf("警告: 使用自签名证书，客户端需要接受不受信任的证书")
		} else {
			// 加载用户提供的证书
			cert, err := tls.LoadX509KeyPair(*clientCertFile, *clientKeyFile)
			if err != nil {
				return nil, fmt.Errorf("加载证书失败: %w", err)
			}
			p.clientTLSConfig = &tls.Config{
				Certificates: []tls.Certificate{cert},
			}
		}
	}

	// 设置日志输出
	logPath := *logFile

	// 如果启用自动日志且未指定日志文件，自动生成文件名
	if *autoLog && logPath == "" {
		// 确保日志目录存在
		if err := os.MkdirAll(*logDir, 0755); err != nil {
			return nil, fmt.Errorf("创建日志目录失败: %w", err)
		}

		// 生成带时间戳的日志文件名
		timestamp := time.Now().Format("20060102-150405")
		logPath = fmt.Sprintf("%s/imap-proxy-%s.log", *logDir, timestamp)
	}

	if logPath != "" {
		// 确保日志文件所在目录存在
		if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
			return nil, fmt.Errorf("创建日志目录失败: %w", err)
		}

		file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("打开日志文件失败: %w", err)
		}
		p.logFile = file

		// 使用带缓冲的写入器，确保日志及时刷新到文件
		// 同时输出到文件和控制台（便于实时查看）
		writers := []io.Writer{
			&flushWriter{file}, // 带刷新的文件写入器
			os.Stdout,
		}
		if p.logger != nil {
			// 如果已经有 logger（自签名证书警告），也输出到 stderr
			writers = append(writers, os.Stderr)
		}
		p.logger = log.New(io.MultiWriter(writers...), "", log.LstdFlags)
	} else {
		if p.logger == nil {
			p.logger = log.New(os.Stdout, "", log.LstdFlags)
		}
	}

	return p, nil
}

// Start 启动代理服务器
func (p *Proxy) Start(ctx context.Context) error {
	listener, err := net.Listen("tcp", p.listenAddr)
	if err != nil {
		return fmt.Errorf("监听失败: %w", err)
	}
	defer listener.Close()

	// 如果启用客户端 TLS，包装为 TLS listener
	if p.clientTLS && p.clientTLSConfig != nil {
		listener = tls.NewListener(listener, p.clientTLSConfig)
		p.logger.Printf("IMAP 透传代理启动（客户端 TLS 模式）")
	} else {
		p.logger.Printf("IMAP 透传代理启动（普通 TCP 模式）")
	}

	p.logger.Printf("监听地址: %s", p.listenAddr)
	p.logger.Printf("目标服务器: %s (TLS: %v)", p.targetAddr, p.useTLS)
	if p.clientTLS {
		p.logger.Printf("客户端连接: TLS (需要客户端配置 SSL/TLS)")
	} else {
		p.logger.Printf("客户端连接: 普通 TCP (客户端应配置为无加密或 STARTTLS)")
	}

	// 显示日志文件路径
	logPath := *logFile
	if *autoLog && logPath == "" {
		timestamp := time.Now().Format("20060102-150405")
		logPath = fmt.Sprintf("%s/imap-proxy-%s.log", *logDir, timestamp)
	}
	if logPath != "" {
		absPath, _ := filepath.Abs(logPath)
		p.logger.Printf("日志文件: %s", absPath)
	} else {
		p.logger.Printf("日志输出: 标准输出（未保存到文件）")
	}

	p.logger.Printf("等待客户端连接...")
	separator := strings.Repeat("=", 80)
	p.logger.Printf("%s", separator)

	// 处理信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 接受连接
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				conn, err := listener.Accept()
				if err != nil {
					select {
					case <-ctx.Done():
						return
					default:
						p.logger.Printf("接受连接失败: %v", err)
						continue
					}
				}

				// 处理每个连接
				go p.handleConnection(conn)
			}
		}
	}()

	// 等待信号
	select {
	case <-sigChan:
		p.logger.Printf("\n收到停止信号，正在关闭...")
		return nil
	case <-ctx.Done():
		return nil
	}
}

// handleConnection 处理客户端连接
func (p *Proxy) handleConnection(clientConn net.Conn) {
	defer clientConn.Close()

	clientAddr := clientConn.RemoteAddr().String()
	connID := fmt.Sprintf("[%s]", time.Now().Format("20060102-150405.000"))

	p.logger.Printf("%s 新客户端连接: %s", connID, clientAddr)
	p.logger.Printf("%s 连接到目标服务器: %s", connID, p.targetAddr)

	// 连接到目标服务器
	var serverConn net.Conn
	var err error

	if p.useTLS {
		// TLS 连接
		tlsConfig := &tls.Config{
			InsecureSkipVerify: p.insecureTLS,
		}
		serverConn, err = tls.DialWithDialer(
			&net.Dialer{Timeout: 10 * time.Second},
			"tcp",
			p.targetAddr,
			tlsConfig,
		)
	} else {
		// 普通 TCP 连接
		serverConn, err = net.DialTimeout("tcp", p.targetAddr, 10*time.Second)
	}

	if err != nil {
		p.logger.Printf("%s 连接目标服务器失败: %v", connID, err)
		return
	}
	defer serverConn.Close()

	p.logger.Printf("%s 已连接到目标服务器", connID)
	p.logger.Printf("%s 开始双向转发数据...", connID)
	p.logger.Printf("%s %s", connID, strings.Repeat("-", 80))

	// 创建双向转发
	var wg sync.WaitGroup
	wg.Add(2)

	// 客户端 -> 服务器
	go func() {
		defer wg.Done()
		p.forwardData(connID, "C->S", clientConn, serverConn)
	}()

	// 服务器 -> 客户端
	go func() {
		defer wg.Done()
		p.forwardData(connID, "S->C", serverConn, clientConn)
	}()

	// 等待转发完成
	wg.Wait()

	p.logger.Printf("%s %s", connID, strings.Repeat("-", 80))
	p.logger.Printf("%s 连接已关闭", connID)
}

// forwardData 转发数据并记录
func (p *Proxy) forwardData(connID, direction string, src, dst net.Conn) {
	// 使用 bufio.Reader 按行读取（IMAP 使用 CRLF 作为行结束符）
	reader := bufio.NewReader(src)
	lineNum := 0

	for {
		// 读取一行（包括 CRLF）
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err != io.EOF {
				if p.verbose {
					p.logger.Printf("%s %s 读取错误: %v", connID, direction, err)
				}
			}
			return
		}

		lineNum++

		// 移除末尾的换行符用于显示
		lineForLog := bytes.TrimRight(line, "\r\n")
		if len(lineForLog) == 0 {
			// 空行，直接转发
			if _, err := dst.Write(line); err != nil {
				return
			}
			continue
		}

		// 记录原始数据（隐藏敏感信息）
		logLine := p.sanitizeLine(lineForLog)
		p.logger.Printf("%s %s [%d] %s", connID, direction, lineNum, string(logLine))

		// 转发原始数据（保持 CRLF）
		if _, err := dst.Write(line); err != nil {
			if p.verbose {
				p.logger.Printf("%s %s 写入失败: %v", connID, direction, err)
			}
			return
		}

		// 如果是详细模式，解析并显示命令
		if p.verbose {
			p.parseAndLogCommand(connID, direction, lineForLog)
		}
	}
}

// sanitizeLine 清理敏感信息
func (p *Proxy) sanitizeLine(line []byte) []byte {
	lineStr := string(line)

	// 隐藏 LOGIN 命令中的密码
	if strings.HasPrefix(lineStr, "LOGIN ") {
		parts := strings.Fields(lineStr)
		if len(parts) >= 3 {
			// 格式: LOGIN username password
			return []byte(fmt.Sprintf("LOGIN %s ***", parts[1]))
		}
	}

	// 隐藏 AUTHENTICATE 命令中的密码（如果可见）
	if strings.HasPrefix(lineStr, "AUTHENTICATE ") {
		parts := strings.Fields(lineStr)
		if len(parts) >= 2 {
			// 只显示认证机制，隐藏后续数据
			return []byte(fmt.Sprintf("AUTHENTICATE %s ***", parts[1]))
		}
	}

	return line
}

// parseAndLogCommand 解析并记录 IMAP 命令
func (p *Proxy) parseAndLogCommand(connID, direction string, line []byte) {
	lineStr := strings.TrimSpace(string(line))
	if len(lineStr) == 0 {
		return
	}

	// 解析 IMAP 命令
	parts := strings.Fields(lineStr)
	if len(parts) == 0 {
		return
	}

	command := parts[0]
	args := ""
	if len(parts) > 1 {
		args = strings.Join(parts[1:], " ")
	}

	// 隐藏敏感信息（密码）
	if command == "LOGIN" && len(parts) >= 3 {
		args = parts[1] + " ***"
	}

	// 记录命令摘要
	if direction == "C->S" {
		p.logger.Printf("%s >>> 命令: %s %s", connID, command, args)
	} else {
		// 服务器响应通常是状态码
		if len(parts) >= 2 {
			status := parts[0]
			message := strings.Join(parts[1:], " ")
			p.logger.Printf("%s <<< 响应: %s %s", connID, status, message)
		}
	}
}

// flushWriter 带刷新的写入器，确保日志及时写入文件
type flushWriter struct {
	file *os.File
}

func (fw *flushWriter) Write(p []byte) (n int, err error) {
	n, err = fw.file.Write(p)
	if err != nil {
		return n, err
	}
	// 每次写入后立即刷新，确保日志及时保存
	if syncErr := fw.file.Sync(); syncErr != nil {
		return n, syncErr
	}
	return n, nil
}

// generateSelfSignedCert 生成自签名证书
func generateSelfSignedCert() (tls.Certificate, error) {
	// 生成私钥
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("生成私钥失败: %w", err)
	}

	// 创建证书模板
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"GoMailZero IMAP Proxy"},
			Country:       []string{"CN"},
			Province:      []string{""},
			Locality:      []string{""},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour), // 1年有效期
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		DNSNames:              []string{"localhost"},
	}

	// 创建证书
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("创建证书失败: %w", err)
	}

	// 编码证书和私钥
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("编码私钥失败: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})

	// 加载证书
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("加载证书失败: %w", err)
	}

	return cert, nil
}

// Close 关闭代理
func (p *Proxy) Close() error {
	if p.logFile != nil {
		return p.logFile.Close()
	}
	return nil
}

func main() {
	flag.Parse()

	proxy, err := NewProxy()
	if err != nil {
		log.Fatalf("创建代理失败: %v", err)
	}
	defer proxy.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := proxy.Start(ctx); err != nil {
		log.Fatalf("代理运行失败: %v", err)
	}
}
