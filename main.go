package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"snishaper/cert"
	"snishaper/proxy"
	"snishaper/sysproxy"
)

var (
	//go:embed templates/*.html
	templateFS embed.FS
	//go:embed templates/*.css
	cssFS embed.FS
	//go:embed templates/*.png
	assetFS embed.FS
)

var (
	Version      = "1.0.0"
	listenAddr   string
	configDir    string
	rulesFile    string
	settingsFile string
	certDir      string
	mode         string
	apiAddr      string
	showVersion  bool
	showHelp     bool
)

// Template cache
var tmpl *template.Template
var pageTemplates = make(map[string]*template.Template)

func initTemplates() {
	var err error

	// Parse base template
	baseTmpl, err := template.New("base").ParseFS(templateFS, "templates/base.html")
	if err != nil {
		log.Fatalf("Failed to parse base template: %v", err)
	}

	// Create page-specific templates
	pages := []string{"index", "dashboard", "proxies", "status", "stats", "logs", "mode", "rules", "rule_edit", "cfpool", "dns", "upstreams", "routing", "settings"}

	for _, page := range pages {
		pageFile := fmt.Sprintf("templates/%s.html", page)
		// Clone base template and add page-specific content
		pageTmpl, err := baseTmpl.Clone()
		if err != nil {
			log.Printf("[Warn] Failed to clone template for %s: %v", page, err)
			continue
		}

		// Parse page-specific content
		pageTmpl, err = pageTmpl.ParseFS(templateFS, pageFile)
		if err != nil {
			log.Printf("[Warn] Failed to parse %s: %v", page, err)
			continue
		}

		pageTemplates[page] = pageTmpl
	}

	// Fallback: use combined templates
	tmpl, err = template.New("base").ParseFS(templateFS, "templates/*.html")
	if err != nil {
		log.Fatalf("Failed to parse templates: %v", err)
	}
}

func getPageTemplate(page string) *template.Template {
	if pt, ok := pageTemplates[page]; ok {
		return pt
	}
	return tmpl
}

type PageData struct {
	Title       string
	CurrentMode string
}

func init() {
	flag.StringVar(&listenAddr, "i", "", "listen address (short: -i)")
	flag.StringVar(&listenAddr, "input", "", "listen address")
	flag.StringVar(&listenAddr, "l", "0.0.0.0:8080", "listen address")
	flag.StringVar(&listenAddr, "listen", "0.0.0.0:8080", "listen address")
	flag.StringVar(&configDir, "c", "", "config directory (short: -c)")
	flag.StringVar(&configDir, "config", "", "config directory")
	flag.StringVar(&rulesFile, "r", "", "rules config file (short: -r)")
	flag.StringVar(&rulesFile, "rules", "", "rules config file")
	flag.StringVar(&settingsFile, "s", "", "settings config file (short: -s)")
	flag.StringVar(&settingsFile, "settings", "", "settings config file")
	flag.StringVar(&certDir, "d", "", "certificate directory (short: -d)")
	flag.StringVar(&certDir, "cert-dir", "", "certificate directory")
	flag.StringVar(&mode, "m", "", "proxy mode: mitm, transparent, tls-rf, quic (short: -m)")
	flag.StringVar(&mode, "mode", "", "proxy mode: mitm, transparent, tls-rf, quic")
	flag.StringVar(&apiAddr, "api", "", "API server address (short: -api)")
	flag.BoolVar(&showVersion, "v", false, "show version (short: -v)")
	flag.BoolVar(&showVersion, "version", false, "show version")
	flag.BoolVar(&showHelp, "h", false, "show help (short: -h)")
	flag.BoolVar(&showHelp, "help", false, "show help")

	flag.Usage = func() {
		fmt.Print(`SniShaper CLI - Cloudflare IP Shaper for Linux

Usage:
  snishaper [OPTIONS]

Options:
`)
		flag.PrintDefaults()
		fmt.Print(`
Examples:
  snishaper                    # Run with default settings
  snishaper -l :8080           # Listen on all interfaces port 8080
  snishaper -c /etc/snishaper  # Use custom config directory
  snishaper -m mitm            # Start in MITM mode
  snishaper -api 0.0.0.0:8081  # API server on port 8081

For more information: https://github.com/SniShaper/snishaper
`)
	}
}

func normalizePath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.ReplaceAll(p, "\\", "/")
	p = strings.TrimSuffix(p, "/")
	return p
}

func windowsToLinuxPath(p string) (string, error) {
	p = normalizePath(p)
	if len(p) < 2 {
		return p, nil
	}
	if p[1] == ':' {
		drive := strings.ToLower(string(p[0]))
		return "/mnt/" + drive + p[2:], nil
	}
	return p, nil
}

func getDefaultConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/etc/snishaper"
	}
	return filepath.Join(home, ".config", "snishaper")
}

func ensureConfigPaths() (string, string, string) {
	cfgDir := configDir
	if cfgDir == "" {
		cfgDir = getDefaultConfigDir()
	}

	if runtime.GOOS == "windows" {
		linuxPath, err := windowsToLinuxPath(cfgDir)
		if err == nil {
			cfgDir = linuxPath
		}
	}

	cfgDir = normalizePath(cfgDir)

	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		log.Fatalf("Failed to create config directory: %v", err)
	}

	rulesPath := rulesFile
	if rulesPath == "" {
		rulesPath = filepath.Join(cfgDir, "rules.json")
	}

	settingsPath := settingsFile
	if settingsPath == "" {
		settingsPath = filepath.Join(cfgDir, "settings.json")
	}

	certPath := certDir
	if certPath == "" {
		certPath = filepath.Join(cfgDir, "certs")
	}

	if err := os.MkdirAll(certPath, 0755); err != nil {
		log.Fatalf("Failed to create cert directory: %v", err)
	}

	copyDefaultRules(rulesPath)

	return rulesPath, settingsPath, certPath
}

func copyDefaultRules(rulesPath string) {
	if _, err := os.Stat(rulesPath); err == nil {
		return
	}

	// Try to copy from rules/config.json in project root
	execDir := getExecutableDir()
	sourceRules := filepath.Join(execDir, "rules", "config.json")

	// Also check current working directory
	if _, err := os.Stat(sourceRules); err != nil {
		cwd, _ := os.Getwd()
		sourceRules = filepath.Join(cwd, "rules", "config.json")
	}

	if data, err := os.ReadFile(sourceRules); err == nil {
		if err := os.WriteFile(rulesPath, data, 0644); err != nil {
			log.Printf("[Warn] Failed to write default rules: %v", err)
		} else {
			log.Printf("[Info] Copied default rules from %s to %s", sourceRules, rulesPath)
		}
	} else {
		log.Printf("[Warn] Default rules file not found at %s, using empty config", sourceRules)
		// Fallback: create minimal config
		minimalConfig := `{
  "site_groups": [],
  "upstreams": [],
  "dns_nodes": [],
  "ech_profiles": []
}`
		if err := os.WriteFile(rulesPath, []byte(minimalConfig), 0644); err != nil {
			log.Printf("[Warn] Failed to create minimal config: %v", err)
		}
	}
}

func getExecutableDir() string {
	ex, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(ex)
}

type CLIApp struct {
	proxyServer  *proxy.ProxyServer
	certManager  *cert.CertManager
	ruleManager  *proxy.RuleManager
	listenAddr   string
	logBuffer    *ringLogWriter
	logCaptureMu sync.RWMutex
	lang         string
	langMu       sync.RWMutex
}

// Language translations for CLI logs
var cliTranslations = map[string]map[string]string{
	"zh": {
		"proxyStarted":     "代理服务器已启动",
		"proxyStopped":     "代理服务器已停止",
		"configReloaded":   "配置已重载",
		"modeChanged":      "模式已切换为: %s",
		"certInstalled":    "证书已安装",
		"newConnection":    "新连接: %s",
		"connectionClosed": "连接关闭: %s",
		"errorOccurred":    "错误: %v",
		"echEnabled":       "ECH已启用",
		"ipShaped":         "IP分流: %s -> %s",
		"healthCheckStart": "健康检查开始",
		"healthCheckDone":  "健康检查完成",
		"invalidIPRemoved": "无效IP已移除: %s",
	},
	"en": {
		"proxyStarted":     "Proxy server started",
		"proxyStopped":     "Proxy server stopped",
		"configReloaded":   "Config reloaded",
		"modeChanged":      "Mode changed to: %s",
		"certInstalled":    "Certificate installed",
		"newConnection":    "New connection: %s",
		"connectionClosed": "Connection closed: %s",
		"errorOccurred":    "Error: %v",
		"echEnabled":       "ECH enabled",
		"ipShaped":         "IP shaped: %s -> %s",
		"healthCheckStart": "Health check started",
		"healthCheckDone":  "Health check completed",
		"invalidIPRemoved": "Invalid IP removed: %s",
	},
}

func (a *CLIApp) GetLang() string {
	a.langMu.RLock()
	defer a.langMu.RUnlock()
	if a.lang == "" {
		return "en"
	}
	return a.lang
}

func (a *CLIApp) SetLang(lang string) {
	a.langMu.Lock()
	defer a.langMu.Unlock()
	if lang == "zh" || lang == "en" {
		a.lang = lang
	}
}

// LogT returns translated log message
func (a *CLIApp) LogT(key string, args ...interface{}) string {
	lang := a.GetLang()
	if trans, ok := cliTranslations[lang]; ok {
		if format, ok := trans[key]; ok {
			if len(args) > 0 {
				return fmt.Sprintf(format, args...)
			}
			return format
		}
	}
	// Fallback to English
	if trans, ok := cliTranslations["en"]; ok {
		if format, ok := trans[key]; ok {
			if len(args) > 0 {
				return fmt.Sprintf(format, args...)
			}
			return format
		}
	}
	return key
}

type ringLogWriter struct {
	mu      sync.Mutex
	lines   []string
	max     int
	pending string
}

func newRingLogWriter(max int) *ringLogWriter {
	if max <= 0 {
		max = 1000
	}
	return &ringLogWriter{
		lines: make([]string, 0, max),
		max:   max,
	}
}

func (w *ringLogWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	text := w.pending + strings.ReplaceAll(string(p), "\r\n", "\n")
	parts := strings.Split(text, "\n")
	if len(parts) == 0 {
		return len(p), nil
	}
	w.pending = parts[len(parts)-1]
	for _, line := range parts[:len(parts)-1] {
		if line == "" {
			continue
		}
		w.lines = append(w.lines, line)
		if len(w.lines) > w.max {
			if cap(w.lines) > w.max*2 {
				newLines := make([]string, w.max)
				copy(newLines, w.lines[len(w.lines)-w.max:])
				w.lines = newLines
			} else {
				w.lines = w.lines[len(w.lines)-w.max:]
			}
		}
	}
	return len(p), nil
}

func (w *ringLogWriter) Snapshot(limit int) []string {
	if limit <= 0 {
		limit = 200
	}
	w.mu.Lock()
	defer w.mu.Unlock()

	total := len(w.lines)
	if total == 0 {
		if w.pending != "" {
			return []string{w.pending}
		}
		return []string{}
	}
	if limit > total {
		limit = total
	}
	start := total - limit
	out := make([]string, limit)
	copy(out, w.lines[start:])
	return out
}

func (w *ringLogWriter) Clear() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.lines = w.lines[:0]
	w.pending = ""
}

func (w *ringLogWriter) AppendLine(line string) {
	if line == "" {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.lines = append(w.lines, line)
	if len(w.lines) > w.max {
		w.lines = w.lines[len(w.lines)-w.max:]
	}
}

func NewCLIApp(rulesPath, settingsPath, certPath, listen string) (*CLIApp, error) {
	app := &CLIApp{
		listenAddr: listen,
		logBuffer:  newRingLogWriter(5000),
	}

	app.ruleManager = proxy.NewRuleManager(settingsPath, rulesPath)
	if err := app.ruleManager.LoadConfig(); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	port := app.ruleManager.GetListenPort()
	if port != "" && listen == "127.0.0.1:8080" {
		listen = "127.0.0.1:" + port
		app.listenAddr = listen
	}

	var err error
	app.certManager, err = cert.InitCertManager(certPath)
	if err != nil {
		return nil, fmt.Errorf("failed to init cert manager: %w", err)
	}

	app.proxyServer = proxy.NewProxyServer(listen)
	app.proxyServer.SetRuleManager(app.ruleManager)
	app.proxyServer.UpdateCloudflareConfig(app.ruleManager.GetCloudflareConfig())
	app.proxyServer.SetCertGenerator(app.certManager)
	app.proxyServer.SetLogCallback(app.appendLog)

	cf := app.ruleManager.GetCloudflareConfig()
	app.proxyServer.UpdateCloudflareConfig(cf)

	return app, nil
}

func (a *CLIApp) appendLog(message string) {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return
	}
	// Translate log messages based on language setting
	translated := a.translateLogMessage(trimmed)
	fmt.Println(translated)
	a.logBuffer.AppendLine(translated)
}

// translateLogMessage translates common log messages
func (a *CLIApp) translateLogMessage(msg string) string {
	lang := a.GetLang()
	if lang == "en" {
		return msg
	}

	// Chinese translations for common log patterns
	translations := map[string]string{
		"Proxy server started":       "代理服务器已启动",
		"Proxy server stopped":       "代理服务器已停止",
		"Config reloaded":            "配置已重新加载",
		"Certificate installed":      "证书已安装",
		"Certificate generated":      "证书已生成",
		"New connection":             "新连接",
		"Connection closed":          "连接已关闭",
		"Connection error":           "连接错误",
		"ECH enabled":                "ECH已启用",
		"ECH disabled":               "ECH已禁用",
		"IP shaped":                  "IP已分流",
		"Health check started":       "健康检查已开始",
		"Health check completed":     "健康检查已完成",
		"Invalid IP removed":         "无效IP已移除",
		"Mode changed to":            "模式已切换为",
		"Error":                      "错误",
		"Warning":                    "警告",
		"Info":                       "信息",
		"Debug":                      "调试",
		"Failed to":                  "失败",
		"Success":                    "成功",
		"Loading":                    "加载中",
		"Saving":                     "保存中",
		"Starting":                   "启动中",
		"Stopping":                   "停止中",
		"Reloading":                  "重新加载中",
		"Creating":                   "创建中",
		"Updating":                   "更新中",
		"Deleting":                   "删除中",
		"Rule added":                 "规则已添加",
		"Rule updated":               "规则已更新",
		"Rule deleted":               "规则已删除",
		"ECH profile added":          "ECH配置已添加",
		"ECH profile updated":        "ECH配置已更新",
		"ECH profile deleted":        "ECH配置已删除",
		"Cloudflare IP pool updated": "Cloudflare IP池已更新",
		"Transparent proxy":          "透明代理",
		"MITM proxy":                 "MITM代理",
		"TLS-RF proxy":               "TLS-RF代理",
		"QUIC proxy":                 "QUIC代理",
		"Unknown":                    "未知",
		"Running":                    "运行中",
		"Stopped":                    "已停止",
		"Enabled":                    "已启用",
		"Disabled":                   "已禁用",
		"Active":                     "活跃",
		"Inactive":                   "非活跃",
		"Pending":                    "等待中",
		"Completed":                  "已完成",
		"Cancelled":                  "已取消",
		"Timeout":                    "超时",
		"Retrying":                   "重试中",
		"Connected":                  "已连接",
		"Disconnected":               "已断开",
		"Reconnecting":               "重新连接中",
		"Listening on":               "监听于",
		"Server started":             "服务器已启动",
		"Server stopped":             "服务器已停止",
		"Request received":           "请求已接收",
		"Response sent":              "响应已发送",
		"Request failed":             "请求失败",
		"Response error":             "响应错误",
		"Upstream":                   "上游",
		"Downstream":                 "下游",
		"Client":                     "客户端",
		"Target":                     "目标",
		"Host":                       "主机",
		"Port":                       "端口",
		"Address":                    "地址",
		"Protocol":                   "协议",
		"Method":                     "方法",
		"Path":                       "路径",
		"Query":                      "查询",
		"Header":                     "头部",
		"Body":                       "主体",
		"Status":                     "状态",
		"Code":                       "代码",
		"Message":                    "消息",
		"Duration":                   "持续时间",
		"Size":                       "大小",
		"Speed":                      "速度",
		"Latency":                    "延迟",
		"Bandwidth":                  "带宽",
		"Traffic":                    "流量",
		"Download":                   "下载",
		"Upload":                     "上传",
		"Total":                      "总计",
		"Average":                    "平均",
		"Peak":                       "峰值",
		"Current":                    "当前",
		"Previous":                   "之前",
		"Next":                       "下一个",
		"First":                      "第一个",
		"Last":                       "最后一个",
		"Index":                      "索引",
		"Count":                      "计数",
		"Limit":                      "限制",
		"Offset":                     "偏移",
		"Page":                       "页",
		"of":                         "的",
		"from":                       "从",
		"to":                         "到",
		"in":                         "在",
		"at":                         "于",
		"on":                         "在",
		"for":                        "为",
		"with":                       "带有",
		"without":                    "不带",
		"by":                         "通过",
		"via":                        "经由",
		"through":                    "穿过",
		"into":                       "进入",
		"onto":                       "到...上",
		"off":                        "关闭",
		"out":                        "外",
		"over":                       "超过",
		"under":                      "低于",
		"above":                      "上方",
		"below":                      "下方",
		"between":                    "之间",
		"among":                      "之中",
		"around":                     "周围",
		"near":                       "附近",
		"far":                        "远",
		"close":                      "近",
		"open":                       "打开",
		"closed":                     "关闭",
		"available":                  "可用",
		"unavailable":                "不可用",
		"valid":                      "有效",
		"invalid":                    "无效",
		"success":                    "成功",
		"failed":                     "失败",
		"complete":                   "完成",
		"incomplete":                 "未完成",
		"ready":                      "就绪",
		"not ready":                  "未就绪",
		"initialized":                "已初始化",
		"uninitialized":              "未初始化",
		"configured":                 "已配置",
		"unconfigured":               "未配置",
		"connected":                  "已连接",
		"disconnected":               "已断开",
		"online":                     "在线",
		"offline":                    "离线",
		"active":                     "活跃",
		"inactive":                   "非活跃",
		"enabled":                    "已启用",
		"disabled":                   "已禁用",
		"started":                    "已启动",
		"stopped":                    "已停止",
		"running":                    "运行中",
		"paused":                     "已暂停",
		"resumed":                    "已恢复",
		"cancelled":                  "已取消",
		"finished":                   "已完成",
		"terminated":                 "已终止",
		"aborted":                    "已中止",
		"skipped":                    "已跳过",
		"ignored":                    "已忽略",
		"accepted":                   "已接受",
		"rejected":                   "已拒绝",
		"approved":                   "已批准",
		"denied":                     "已拒绝",
		"granted":                    "已授权",
		"revoked":                    "已撤销",
		"allowed":                    "已允许",
		"blocked":                    "已阻止",
		"permitted":                  "已许可",
		"forbidden":                  "已禁止",
		"authorized":                 "已授权",
		"unauthorized":               "未授权",
		"authenticated":              "已认证",
		"unauthenticated":            "未认证",
		"verified":                   "已验证",
		"unverified":                 "未验证",
		"confirmed":                  "已确认",
		"unconfirmed":                "未确认",
		"validated":                  "已校验",
		"unvalidated":                "未校验",
		"checked":                    "已检查",
		"unchecked":                  "未检查",
		"tested":                     "已测试",
		"untested":                   "未测试",
		"passed":                     "通过",
		"succeeded":                  "成功",
		"errored":                    "出错",
		"completed":                  "已完成",
		"done":                       "完成",
		"pending":                    "等待中",
		"processing":                 "处理中",
		"waiting":                    "等待中",
		"loading":                    "加载中",
		"saving":                     "保存中",
		"deleting":                   "删除中",
		"updating":                   "更新中",
		"creating":                   "创建中",
		"reading":                    "读取中",
		"writing":                    "写入中",
		"uploading":                  "上传中",
		"downloading":                "下载中",
		"syncing":                    "同步中",
		"importing":                  "导入中",
		"exporting":                  "导出中",
		"converting":                 "转换中",
		"compiling":                  "编译中",
		"building":                   "构建中",
		"deploying":                  "部署中",
		"installing":                 "安装中",
		"uninstalling":               "卸载中",
		"upgrading":                  "升级中",
		"downgrading":                "降级中",
		"migrating":                  "迁移中",
		"backing up":                 "备份中",
		"restoring":                  "恢复中",
		"scanning":                   "扫描中",
		"analyzing":                  "分析中",
		"calculating":                "计算中",
		"generating":                 "生成中",
		"rendering":                  "渲染中",
		"fetching":                   "获取中",
		"retrieving":                 "检索中",
		"searching":                  "搜索中",
		"filtering":                  "过滤中",
		"sorting":                    "排序中",
		"grouping":                   "分组中",
		"aggregating":                "聚合中",
		"summarizing":                "汇总中",
		"reporting":                  "报告生成中",
		"logging":                    "日志记录中",
		"monitoring":                 "监控中",
		"watching":                   "监视中",
		"observing":                  "观察中",
		"tracking":                   "追踪中",
		"recording":                  "记录中",
		"storing":                    "存储中",
		"caching":                    "缓存中",
		"buffering":                  "缓冲中",
		"streaming":                  "流式传输中",
		"broadcasting":               "广播中",
		"publishing":                 "发布中",
		"subscribing":                "订阅中",
		"unsubscribing":              "取消订阅中",
		"registering":                "注册中",
		"unregistering":              "注销中",
		"enrolling":                  "登记中",
		"unenrolling":                "取消登记中",
		"activating":                 "激活中",
		"deactivating":               "停用中",
		"enabling":                   "启用中",
		"disabling":                  "禁用中",
		"locking":                    "锁定中",
		"unlocking":                  "解锁中",
		"encrypting":                 "加密中",
		"decrypting":                 "解密中",
		"compressing":                "压缩中",
		"decompressing":              "解压缩中",
		"packaging":                  "打包中",
		"unpackaging":                "解包中",
		"archiving":                  "归档中",
		"unarchiving":                "解归档中",
		"zipping":                    "压缩中",
		"unzipping":                  "解压中",
		"extracting":                 "提取中",
		"inserting":                  "插入中",
		"removing":                   "移除中",
		"adding":                     "添加中",
		"subtracting":                "减去中",
		"multiplying":                "乘以中",
		"dividing":                   "除以中",
		"computing":                  "计算中",
		"evaluating":                 "评估中",
		"executing":                  "执行中",
		"performing":                 "执行中",
		"operating":                  "操作中",
		"functioning":                "运行中",
		"working":                    "工作中",
		"starting":                   "启动中",
		"stopping":                   "停止中",
		"restarting":                 "重启中",
		"shutting down":              "关闭中",
		"booting":                    "引导中",
		"initializing":               "初始化中",
		"resetting":                  "重置中",
		"clearing":                   "清除中",
		"flushing":                   "刷新中",
		"purging":                    "清理中",
		"cleaning":                   "清理中",
		"optimizing":                 "优化中",
		"tuning":                     "调优中",
		"adjusting":                  "调整中",
		"configuring":                "配置中",
		"setting up":                 "设置中",
		"preparing":                  "准备中",
		"planning":                   "计划中",
		"scheduling":                 "调度中",
		"queuing":                    "排队中",
		"dispatching":                "分发中",
		"routing":                    "路由中",
		"forwarding":                 "转发中",
		"redirecting":                "重定向中",
		"proxying":                   "代理中",
		"balancing":                  "负载均衡中",
		"distributing":               "分发中",
		"allocating":                 "分配中",
		"assigning":                  "分配中",
		"reserving":                  "预留中",
		"releasing":                  "释放中",
		"freeing":                    "释放中",
		"recycling":                  "回收中",
		"reclaiming":                 "回收中",
		"garbage collecting":         "垃圾回收中",
	}

	// Try exact match first
	if translated, ok := translations[msg]; ok {
		return translated
	}

	// Try prefix matching for dynamic messages
	for en, zh := range translations {
		if strings.HasPrefix(msg, en) {
			// Replace the prefix with translated version
			return zh + msg[len(en):]
		}
	}

	// Return original if no translation found
	return msg
}

func (a *CLIApp) Start() error {
	return a.proxyServer.Start()
}

func (a *CLIApp) Stop() error {
	return a.proxyServer.Stop()
}

func (a *CLIApp) IsRunning() bool {
	return a.proxyServer.IsRunning()
}

func (a *CLIApp) GetStats() (int64, int64) {
	down, up, _ := a.proxyServer.GetStats()
	return down, up
}

func (a *CLIApp) ReloadConfig() error {
	if err := a.ruleManager.LoadConfig(); err != nil {
		return err
	}
	a.proxyServer.SetRuleManager(a.ruleManager)
	a.proxyServer.UpdateCloudflareConfig(a.ruleManager.GetCloudflareConfig())

	// If auto update is enabled, fetch IPs immediately
	cfg := a.ruleManager.GetCloudflareConfig()
	if cfg.AutoUpdate {
		a.appendLog("[Cloudflare] Auto update enabled, fetching initial IPs...")
		go a.RefreshCloudflareIPPool()
	}
	return nil
}

func (a *CLIApp) RefreshCloudflareIPPool() {
	cfg := a.ruleManager.GetCloudflareConfig()
	ips, err := proxy.FetchCloudflareIPs(cfg.APIKey)
	if err != nil {
		log.Printf("[Cloudflare] Failed to fetch preferred IPs: %v", err)
		a.appendLog("[error] Cloudflare 优选 IP 获取失败: " + err.Error())
		return
	}

	if len(ips) > 0 {
		log.Printf("[Cloudflare] Successfully fetched %d preferred IPs", len(ips))
		a.appendLog(fmt.Sprintf("[success] 成功获取 %d 个 Cloudflare 优选 IP", len(ips)))

		a.proxyServer.UpdateCloudflareIPPool(ips)
		// Persist: sync to config file
		cfg.PreferredIPs = ips
		_ = a.ruleManager.UpdateCloudflareConfig(cfg)
	}
}

func (a *CLIApp) ReloadCertificate() error {
	cm, err := cert.InitCertManager(a.certManager.GetCAInstallStatus().CertPath)
	if err != nil {
		return err
	}
	a.certManager = cm
	a.proxyServer.SetCertGenerator(a.certManager)
	a.proxyServer.ClearCertCache()
	return nil
}

func (a *CLIApp) GetMode() string {
	return a.proxyServer.GetMode()
}

func (a *CLIApp) SetMode(newMode string) error {
	return a.proxyServer.SetMode(newMode)
}

func (a *CLIApp) GetLogs(limit int) []string {
	return a.logBuffer.Snapshot(limit)
}

func (a *CLIApp) ClearLogs() {
	a.logBuffer.Clear()
}

func printBanner() {
	fmt.Println(`
╔════════════════════════════════════════════════════════════════╗
║           SniShaper CLI v` + Version + `                    ║
║        Cloudflare IP Shaper - Linux Edition       ║
╚═════════════════════════════════════════════════════════════════╝`)
}

type statsDisplay struct {
	ticker   *time.Ticker
	done     chan struct{}
	lastIn   int64
	lastOut  int64
	lastTick time.Time
}

func newStatsDisplay() *statsDisplay {
	return &statsDisplay{
		ticker:   time.NewTicker(1 * time.Second),
		done:     make(chan struct{}),
		lastIn:   0,
		lastOut:  0,
		lastTick: time.Now(),
	}
}

func (sd *statsDisplay) start(app *CLIApp) {
	go func() {
		for {
			select {
			case <-sd.ticker.C:
				if app.IsRunning() {
					currentIn, currentOut := app.GetStats()
					now := time.Now()
					duration := now.Sub(sd.lastTick).Seconds()

					var downSpeed, upSpeed float64
					if duration > 0 {
						downSpeed = float64(currentIn-sd.lastIn) / duration
						upSpeed = float64(currentOut-sd.lastOut) / duration
					}

					if downSpeed > 0 || upSpeed > 0 {
						fmt.Printf("\r[Stats] ↓ %s/s  ↑ %s/s    ",
							formatBytes(int64(downSpeed)),
							formatBytes(int64(upSpeed)))
					}

					sd.lastIn = currentIn
					sd.lastOut = currentOut
					sd.lastTick = now
				}
			case <-sd.done:
				return
			}
		}
	}()
}

func (sd *statsDisplay) stop() {
	sd.ticker.Stop()
	close(sd.done)
}

func formatBytes(n int64) string {
	if n < 0 {
		n = 0
	}
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for n >= div && exp < 4 {
		div *= unit
		exp++
	}
	div /= unit
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGT"[exp])
}

type APIServer struct {
	app           *CLIApp
	server        *http.Server
	linuxFeatures *LinuxFeaturesManager
}

func NewAPIServer(addr string, app *CLIApp) *APIServer {
	mux := http.NewServeMux()

	s := &APIServer{app: app}

	if runtime.GOOS == "linux" {
		apiPort := 5173
		if parts := strings.Split(addr, ":"); len(parts) == 2 {
			if p, err := strconv.Atoi(parts[1]); err == nil {
				apiPort = p
			}
		}
		s.linuxFeatures = NewLinuxFeaturesManager(app, app.ruleManager, app.proxyServer, apiPort)
		s.linuxFeatures.RegisterRoutes(mux)
	}

	mux.HandleFunc("/style.css", s.handleCSS)
	mux.HandleFunc("/logo.png", s.handleLogo)
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/dashboard", s.handleDashboard)
	mux.HandleFunc("/proxies", s.handleProxies)
	mux.HandleFunc("/status", s.handleStatus)
	mux.HandleFunc("/stats", s.handleStats)
	mux.HandleFunc("/logs", s.handleLogs)
	mux.HandleFunc("/mode", s.handleMode)
	mux.HandleFunc("/lang", s.handleLang)
	mux.HandleFunc("/reload", s.handleReload)
	mux.HandleFunc("/stop", s.handleStop)
	mux.HandleFunc("/rules", s.handleRules)
	mux.HandleFunc("/rules/edit", s.handleRuleEdit)
	mux.HandleFunc("/dns", s.handleDNS)
	mux.HandleFunc("/dns/test", s.handleDNSTest)
	mux.HandleFunc("/upstreams", s.handleUpstreams)
	mux.HandleFunc("/routing", s.handleRouting)
	mux.HandleFunc("/cert", s.handleCert)
	mux.HandleFunc("/config/export", s.handleExportConfig)
	mux.HandleFunc("/config/import", s.handleImportConfig)
	mux.HandleFunc("/cfpool", s.handleCFPool)
	mux.HandleFunc("/settings", s.handleSettings)
	mux.HandleFunc("/server/config", s.handleServerConfig)
	mux.HandleFunc("/server/test", s.handleServerTest)
	mux.HandleFunc("/ech/profiles", s.handleECHProfiles)
	mux.HandleFunc("/proxy/start", s.handleProxyStart)
	mux.HandleFunc("/proxy/stop", s.handleProxyStop)
	mux.HandleFunc("/sysproxy/enable", s.handleSysProxyEnable)
	mux.HandleFunc("/sysproxy/disable", s.handleSysProxyDisable)
	mux.HandleFunc("/tun/start", s.handleTUNStart)
	mux.HandleFunc("/tun/stop", s.handleTUNStop)
	mux.HandleFunc("/tun/status", s.handleTUNStatus)
	mux.HandleFunc("/api/doh", s.handleDoH)

	s.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	return s
}

func (s *APIServer) handleCSS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css")
	data, err := cssFS.ReadFile("templates/style.css")
	if err != nil {
		http.Error(w, "CSS not found", 404)
		return
	}
	w.Write(data)
}

func (s *APIServer) handleLogo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/png")
	data, err := assetFS.ReadFile("templates/logo.png")
	if err != nil {
		http.Error(w, "Logo not found", 404)
		return
	}
	w.Write(data)
}

func (s *APIServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	// 重定向到仪表盘页面
	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

func (s *APIServer) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Accept") == "application/json" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"running":     s.app.IsRunning(),
			"mode":        s.app.GetMode(),
			"listen_addr": s.app.listenAddr,
			"version":     Version,
		})
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := PageData{Title: "Dashboard"}
	if err := getPageTemplate("dashboard").ExecuteTemplate(w, "base", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *APIServer) handleProxies(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Accept") == "application/json" {
		nodes := s.app.ruleManager.GetDNSNodes()
		json.NewEncoder(w).Encode(map[string]interface{}{
			"nodes": nodes,
			"mode":  s.app.GetMode(),
		})
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := PageData{Title: "Proxies"}
	if err := getPageTemplate("proxies").ExecuteTemplate(w, "base", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *APIServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Accept") == "application/json" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"running":    s.app.proxyServer.IsRunning(),
			"mode":       s.app.GetMode(),
			"listenAddr": s.app.listenAddr,
			"version":    Version,
		})
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := PageData{Title: "Status"}
	if err := getPageTemplate("status").ExecuteTemplate(w, "base", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *APIServer) handleStats(w http.ResponseWriter, r *http.Request) {
	down, up := s.app.GetStats()
	if r.Header.Get("Accept") == "application/json" {
		json.NewEncoder(w).Encode(map[string]int64{"download": down, "upload": up})
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := PageData{Title: "Statistics"}
	if err := getPageTemplate("stats").ExecuteTemplate(w, "base", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *APIServer) handleLogs(w http.ResponseWriter, r *http.Request) {
	limit := 100
	fmt.Fscanf(r.Body, "%d", &limit)
	logs := s.app.GetLogs(limit)
	if r.Header.Get("Accept") == "application/json" {
		json.NewEncoder(w).Encode(map[string][]string{"logs": logs})
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := PageData{Title: "Logs"}
	if err := getPageTemplate("logs").ExecuteTemplate(w, "base", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *APIServer) handleMode(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if r.Header.Get("Accept") == "application/json" {
			json.NewEncoder(w).Encode(map[string]string{"mode": s.app.GetMode()})
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		data := PageData{Title: "Mode", CurrentMode: s.app.GetMode()}
		if err := getPageTemplate("mode").ExecuteTemplate(w, "base", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	case http.MethodPost:
		m := r.URL.Query().Get("m")
		if m != "" {
			if err := s.app.SetMode(m); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			fmt.Fprintf(w, "Mode set to %s", m)
		}
	}
}

func (s *APIServer) handleReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	if err := s.app.ReloadConfig(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	fmt.Fprint(w, "Config reloaded")
}

func (s *APIServer) handleLang(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if r.Header.Get("Accept") == "application/json" {
			json.NewEncoder(w).Encode(map[string]string{"lang": s.app.GetLang()})
			return
		}
		fmt.Fprint(w, s.app.GetLang())
	case http.MethodPost:
		lang := r.URL.Query().Get("l")
		if lang != "" {
			s.app.SetLang(lang)
			if r.Header.Get("Accept") == "application/json" {
				json.NewEncoder(w).Encode(map[string]string{"lang": lang, "status": "ok"})
				return
			}
			fmt.Fprintf(w, "Language set to %s", lang)
		}
	}
}

func (s *APIServer) handleStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	fmt.Fprint(w, "Shutting down...")
	go func() {
		s.app.Stop()
		os.Exit(0)
	}()
}

func (s *APIServer) handleRules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		siteGroups := s.app.ruleManager.GetSiteGroups()
		if r.Header.Get("Accept") == "application/json" {
			json.NewEncoder(w).Encode(map[string]interface{}{"rules": siteGroups})
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		data := PageData{Title: "Rules"}
		if err := getPageTemplate("rules").ExecuteTemplate(w, "base", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	case http.MethodPost:
		var rule proxy.SiteGroup
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if err := s.app.ruleManager.AddSiteGroup(rule); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "Missing id", 400)
			return
		}
		if err := s.app.ruleManager.DeleteSiteGroup(id); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func (s *APIServer) handleRuleEdit(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		data := PageData{Title: "Edit Rule"}
		if err := getPageTemplate("rule_edit").ExecuteTemplate(w, "base", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	if r.Method == http.MethodPost {
		var rule proxy.SiteGroup
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if err := s.app.ruleManager.UpdateSiteGroup(rule); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func (s *APIServer) handleDNS(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if r.URL.Query().Get("test") != "" {
			id := r.URL.Query().Get("id")
			if id == "" {
				http.Error(w, "Missing id", 400)
				return
			}
			nodes := s.app.ruleManager.GetDNSNodes()
			for _, node := range nodes {
				if node.ID == id {
					start := time.Now()
					err := testDNSNode(node)
					latency := time.Since(start).Milliseconds()
					if err != nil {
						json.NewEncoder(w).Encode(map[string]interface{}{
							"success": false,
							"error":   err.Error(),
							"latency": latency,
						})
					} else {
						json.NewEncoder(w).Encode(map[string]interface{}{
							"success": true,
							"latency": latency,
						})
					}
					return
				}
			}
			http.Error(w, "Node not found", 404)
			return
		}
		nodes := s.app.ruleManager.GetDNSNodes()
		if r.Header.Get("Accept") == "application/json" {
			json.NewEncoder(w).Encode(map[string]interface{}{"nodes": nodes})
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		data := PageData{Title: "DNS"}
		if err := getPageTemplate("dns").ExecuteTemplate(w, "base", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	case http.MethodPost:
		if r.URL.Query().Get("test") != "" {
			id := r.URL.Query().Get("id")
			if id == "" {
				http.Error(w, "Missing id", 400)
				return
			}
			nodes := s.app.ruleManager.GetDNSNodes()
			for _, node := range nodes {
				if node.ID == id {
					start := time.Now()
					err := testDNSNode(node)
					latency := time.Since(start).Milliseconds()
					if err != nil {
						json.NewEncoder(w).Encode(map[string]interface{}{
							"success": false,
							"error":   err.Error(),
							"latency": latency,
						})
					} else {
						json.NewEncoder(w).Encode(map[string]interface{}{
							"success": true,
							"latency": latency,
						})
					}
					return
				}
			}
			http.Error(w, "Node not found", 404)
			return
		}
		if r.URL.Query().Get("target") != "" {
			id := r.URL.Query().Get("id")
			target := r.URL.Query().Get("target")
			if id == "" || target == "" {
				http.Error(w, "Missing id or target", 400)
				return
			}
			targetIdx, err := strconv.Atoi(target)
			if err != nil {
				http.Error(w, "Invalid target", 400)
				return
			}
			if err := s.app.ruleManager.SetDNSNodePriority(id, targetIdx); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
			return
		}
		var node proxy.DNSNode
		if err := json.NewDecoder(r.Body).Decode(&node); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if err := s.app.ruleManager.AddDNSNode(node); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	case http.MethodPut:
		var node proxy.DNSNode
		if err := json.NewDecoder(r.Body).Decode(&node); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if err := s.app.ruleManager.UpdateDNSNode(node); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "Missing id", 400)
			return
		}
		if err := s.app.ruleManager.DeleteDNSNode(id); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func testDNSNode(node proxy.DNSNode) error {
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: node.CertVerify.AllowUnknownAuthority,
			},
		},
	}
	resp, err := client.Get(node.URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("DNS node returned status %d", resp.StatusCode)
	}
	return nil
}

func (s *APIServer) handleDNSTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing id", 400)
		return
	}
	nodes := s.app.ruleManager.GetDNSNodes()
	for _, node := range nodes {
		if node.ID == id {
			resolver := s.app.proxyServer.GetDoHResolver()
			if resolver == nil {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"error":   "Resolver not initialized",
				})
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			start := time.Now()
			ips, err := resolver.TestNode(ctx, node)
			elapsed := time.Since(start)
			if err != nil {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"error":   err.Error(),
					"latency": fmt.Sprintf("%dms", elapsed.Milliseconds()),
				})
			} else {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"success": len(ips) > 0,
					"ips":     ips,
					"latency": fmt.Sprintf("%dms", elapsed.Milliseconds()),
				})
			}
			return
		}
	}
	http.Error(w, "Node not found", 404)
}

func (s *APIServer) handleUpstreams(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		upstreams := s.app.ruleManager.GetUpstreams()
		if r.Header.Get("Accept") == "application/json" {
			json.NewEncoder(w).Encode(map[string]interface{}{"upstreams": upstreams})
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		data := PageData{Title: "Upstreams"}
		if err := getPageTemplate("upstreams").ExecuteTemplate(w, "base", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	case http.MethodPost:
		var upstream proxy.Upstream
		if err := json.NewDecoder(r.Body).Decode(&upstream); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if err := s.app.ruleManager.AddUpstream(upstream); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	case http.MethodPut:
		var upstream proxy.Upstream
		if err := json.NewDecoder(r.Body).Decode(&upstream); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if err := s.app.ruleManager.UpdateUpstream(upstream); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "Missing id", 400)
			return
		}
		if err := s.app.ruleManager.DeleteUpstream(id); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func (s *APIServer) handleRouting(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/routing/status" && r.Method == http.MethodGet {
		config := s.app.ruleManager.GetAutoRoutingConfig()
		domainCount := 0
		router := s.app.ruleManager.GetAutoRouter()
		if router != nil {
			gfwList := router.GetGFWList()
			if gfwList != nil {
				domainCount = gfwList.Count()
			}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"mode":         string(config.Mode),
			"domain_count": domainCount,
			"last_update":  config.LastUpdate,
		})
		return
	}

	if r.URL.Path == "/routing/refresh" && r.Method == http.MethodPost {
		count, err := s.app.ruleManager.RefreshGFWList()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"count":  count,
		})
		return
	}

	switch r.Method {
	case http.MethodGet:
		config := s.app.ruleManager.GetAutoRoutingConfig()
		if r.Header.Get("Accept") == "application/json" {
			json.NewEncoder(w).Encode(map[string]interface{}{"config": config})
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		data := PageData{Title: "Routing"}
		if err := getPageTemplate("routing").ExecuteTemplate(w, "base", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	case http.MethodPost:
		var config proxy.AutoRoutingConfig
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if err := s.app.ruleManager.UpdateAutoRoutingConfig(config); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func (s *APIServer) handleCert(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		action := r.URL.Query().Get("action")
		if action == "path" {
			json.NewEncoder(w).Encode(map[string]string{"path": certDir})
			return
		}
		if action == "export" {
			pem, err := s.app.certManager.ExportCert()
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			w.Header().Set("Content-Type", "application/x-x509-ca-cert")
			w.Header().Set("Content-Disposition", "attachment; filename=snishaper-ca.pem")
			w.Write(pem)
			return
		}
		if r.Header.Get("Accept") == "application/json" {
			status := s.app.certManager.GetCAInstallStatus()
			json.NewEncoder(w).Encode(status)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		data := PageData{Title: "Certificate Management"}
		if err := getPageTemplate("cert").ExecuteTemplate(w, "base", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	case http.MethodPost:
		action := r.URL.Query().Get("action")
		if action == "regenerate" {
			if err := s.app.certManager.RegenerateCA(); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			s.app.proxyServer.ClearCertCache()
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		}
		if action == "open_dir" {
			json.NewEncoder(w).Encode(map[string]string{"status": "ok", "path": certDir})
		}
	}
}

func (s *APIServer) handleExportConfig(w http.ResponseWriter, r *http.Request) {
	data, err := s.app.ruleManager.ExportConfig()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=snishaper-config.json")
	w.Write([]byte(data))
}

func (s *APIServer) handleImportConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	file, _, err := r.FormFile("config")
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	if err := s.app.ruleManager.ImportConfig(string(data)); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *APIServer) handleCFPool(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		config := s.app.ruleManager.GetCloudflareConfig()
		if r.Header.Get("Accept") == "application/json" {
			ips := s.app.proxyServer.GetCFPoolIPs()
			ipList := make([]map[string]interface{}, 0)
			if ips != nil {
				for _, ip := range ips {
					ipList = append(ipList, map[string]interface{}{
						"address": ip.IP,
						"latency": ip.Latency,
						"valid":   ip.Failures < 3,
					})
				}
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"config": config,
				"ips":    ipList,
			})
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		data := PageData{Title: "CF Pool"}
		if err := getPageTemplate("cfpool").ExecuteTemplate(w, "base", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	case http.MethodPost:
		action := r.URL.Query().Get("action")
		switch action {
		case "fetch":
			go s.app.RefreshCloudflareIPPool()
			json.NewEncoder(w).Encode(map[string]string{"status": "fetching"})
		case "healthcheck":
			s.app.proxyServer.TriggerCFHealthCheck()
			json.NewEncoder(w).Encode(map[string]string{"status": "checking"})
		case "removeinvalid":
			s.app.proxyServer.RemoveInvalidCFIPs()
			json.NewEncoder(w).Encode(map[string]string{"status": "removed"})
		default:
			var config proxy.CloudflareConfig
			if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			s.app.ruleManager.UpdateCloudflareConfig(config)
			s.app.proxyServer.UpdateCloudflareConfig(config)
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		}
	}
}

func (s *APIServer) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if r.Header.Get("Accept") == "application/json" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"tun":             s.app.ruleManager.GetTUNConfig(),
				"cert":            s.app.certManager.GetCAInstallStatus(),
				"host":            s.app.ruleManager.GetServerHost(),
				"auth":            s.app.ruleManager.GetServerAuth(),
				"port":            s.app.ruleManager.GetListenPort(),
				"auto_start":      s.app.ruleManager.GetAutoStart(),
				"auto_proxy":      s.app.ruleManager.GetAutoProxy(),
				"ip_stats":        s.app.proxyServer.GetCFPoolIPs(),
				"installed_certs": s.app.certManager.GetInstalledCerts(),
			})
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		data := PageData{Title: "Settings"}
		if err := getPageTemplate("settings").ExecuteTemplate(w, "base", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	case http.MethodPost:
		action := r.URL.Query().Get("action")
		switch action {
		case "tun":
			var cfg proxy.TUNConfig
			if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			s.app.ruleManager.UpdateTUNConfig(cfg)
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		case "general":
			var settings map[string]string
			if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			host := settings["host"]
			auth := settings["auth"]
			port := settings["port"]
			if host != "" || auth != "" {
				s.app.ruleManager.UpdateServerConfig(host, auth)
			}
			if port != "" {
				s.app.ruleManager.SetListenPort(port)
			}
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		}
	}
}

func (s *APIServer) handleServerConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		json.NewEncoder(w).Encode(map[string]interface{}{
			"host": s.app.ruleManager.GetServerHost(),
			"auth": s.app.ruleManager.GetServerAuth(),
		})
	case http.MethodPost:
		var req struct {
			Host string `json:"host"`
			Auth string `json:"auth"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		s.app.ruleManager.UpdateServerConfig(req.Host, req.Auth)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func (s *APIServer) handleServerTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	host := s.app.ruleManager.GetServerHost()
	auth := s.app.ruleManager.GetServerAuth()
	if host == "" || auth == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Server host or auth not configured",
		})
		return
	}
	testURL := "https://" + host + "/" + auth + "/test"
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(testURL)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}
	defer resp.Body.Close()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"status":  resp.StatusCode,
	})
}

func (s *APIServer) handleECHProfiles(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		profiles := s.app.ruleManager.GetECHProfiles()
		json.NewEncoder(w).Encode(map[string]interface{}{"profiles": profiles})
	case http.MethodPost:
		var profile proxy.ECHProfile
		if err := json.NewDecoder(r.Body).Decode(&profile); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if err := s.app.ruleManager.UpsertECHProfile(profile); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "Missing id", 400)
			return
		}
		if err := s.app.ruleManager.DeleteECHProfile(id); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func (s *APIServer) handleProxyStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	// 启动代理服务器
	if err := s.app.proxyServer.Start(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}

func (s *APIServer) handleProxyStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	log.Printf("[ProxyStop] Stopping proxy server...")
	// 先停止系统代理（清理iptables规则）
	if runtime.GOOS == "linux" && s.linuxFeatures != nil {
		if err := s.linuxFeatures.DisableSysProxy(); err != nil {
			log.Printf("[ProxyStop] Failed to disable system proxy: %v", err)
			// 继续停止代理服务器，不返回错误
		} else {
			log.Printf("[ProxyStop] System proxy disabled")
		}
	}
	// 停止代理服务器前关闭DoH
	if proxy.DoHEnabled {
		proxy.DoHEnabled = false
		log.Printf("[ProxyStop] DoH disabled")
	}

	// 停止代理服务器，关闭8080端口
	log.Printf("[ProxyStop] Proxy running before stop: %v", s.app.proxyServer.IsRunning())
	if err := s.app.proxyServer.Stop(); err != nil {
		log.Printf("[ProxyStop] Failed to stop proxy: %v", err)
		http.Error(w, err.Error(), 500)
		return
	}
	log.Printf("[ProxyStop] Proxy running after stop: %v", s.app.proxyServer.IsRunning())
	json.NewEncoder(w).Encode(map[string]string{"status": "stopped"})
}

func (s *APIServer) handleSysProxyEnable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	if runtime.GOOS == "linux" && s.linuxFeatures != nil {
		port := s.app.ruleManager.GetListenPort()
		if port == "" {
			port = "8080"
		}
		p, err := strconv.Atoi(port)
		if err != nil {
			p = 8080
		}
		if err := s.linuxFeatures.EnableSysProxy(p); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "enabled"})
}

func (s *APIServer) handleSysProxyDisable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	if runtime.GOOS == "linux" && s.linuxFeatures != nil {
		if err := s.linuxFeatures.DisableSysProxy(); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "disabled"})
}

func (s *APIServer) handleTUNStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	if runtime.GOOS == "linux" && s.linuxFeatures != nil {
		cfg := s.app.ruleManager.GetTUNConfig()
		port := s.app.ruleManager.GetListenPort()
		if port == "" {
			port = "8080"
		}
		p, err := strconv.Atoi(port)
		if err != nil {
			p = 8080
		}
		if err := s.linuxFeatures.StartTUN(cfg, p); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}

func (s *APIServer) handleTUNStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	if runtime.GOOS == "linux" && s.linuxFeatures != nil {
		if err := s.linuxFeatures.StopTUN(); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "stopped"})
}

func (s *APIServer) handleTUNStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", 405)
		return
	}
	cfg := s.app.ruleManager.GetTUNConfig()
	status := map[string]interface{}{
		"running": false,
		"config":  cfg,
	}
	if runtime.GOOS == "linux" && s.linuxFeatures != nil {
		tunStatus := s.linuxFeatures.GetTUNStatus(cfg)
		status["running"] = tunStatus.Running
	}
	json.NewEncoder(w).Encode(status)
}

func (s *APIServer) handleDoH(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		json.NewEncoder(w).Encode(map[string]bool{"enabled": proxy.DoHEnabled})
	case http.MethodPost:
		action := r.URL.Query().Get("action")
		if action == "enable" {
			// 开启DoH必须先开启代理，否则系统代理设置会导致访问错误
			if !s.app.proxyServer.IsRunning() {
				http.Error(w, "DoH can only be enabled when proxy is running", http.StatusBadRequest)
				return
			}
			proxy.DoHEnabled = true
			json.NewEncoder(w).Encode(map[string]string{"status": "enabled"})
		} else if action == "disable" {
			proxy.DoHEnabled = false
			json.NewEncoder(w).Encode(map[string]string{"status": "disabled"})
		} else {
			http.Error(w, "Invalid action", http.StatusBadRequest)
		}
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *APIServer) Start() error {
	log.Printf("[Debug] API server starting on %s", s.server.Addr)
	err := s.server.ListenAndServe()
	log.Printf("[Debug] API server stopped: %v", err)
	return err
}

func (s *APIServer) Stop() error {
	return s.server.Shutdown(context.Background())
}

func waitForSignal(app *CLIApp) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	fmt.Printf("\n[Info] Received signal: %v\n", sig)
	fmt.Println("[Info] Shutting down...")

	if runtime.GOOS == "linux" {
		sysproxy.CleanupIptablesOnExit()
	}

	if err := app.Stop(); err != nil {
		log.Printf("[Error] Stop proxy failed: %v", err)
	}

	fmt.Println("[Info] Goodbye!")
}

func startInteractiveConsole(app *CLIApp, apiServer *APIServer, linuxFeatures *LinuxFeaturesManager) {
	go func() {
		reader := bufio.NewReader(os.Stdin)
		for {
			fmt.Print("\n> ")
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(strings.ToLower(input))

			switch input {
			case "1", "proxy":
				if app.IsRunning() {
					app.Stop()
					fmt.Println("[Console] Proxy stopped")
				} else {
					app.Start()
					fmt.Println("[Console] Proxy started")
				}
			case "2", "sysproxy":
				if linuxFeatures != nil {
					status := linuxFeatures.GetSysProxyStatus()
					if status.Enabled {
						linuxFeatures.DisableSysProxy()
						fmt.Println("[Console] System proxy disabled, all traffic now goes direct")
					} else {
						port := app.ruleManager.GetListenPort()
						if port == "" {
							port = "8080"
						}
						p, _ := strconv.Atoi(port)
						if err := linuxFeatures.EnableSysProxy(p); err != nil {
							fmt.Printf("[Console] Failed to enable system proxy: %v\n", err)
						} else {
							fmt.Println("[Console] System proxy enabled")
						}
					}
				} else {
					fmt.Println("[Console] System proxy not available on this platform")
				}
			case "3", "tun":
				if linuxFeatures != nil {
					cfg := app.ruleManager.GetTUNConfig()
					tunStatus := linuxFeatures.GetTUNStatus(cfg)
					if tunStatus.Running {
						linuxFeatures.StopTUN()
						fmt.Println("[Console] TUN stopped")
					} else {
						port := app.ruleManager.GetListenPort()
						if port == "" {
							port = "8080"
						}
						p, _ := strconv.Atoi(port)
						if err := linuxFeatures.StartTUN(cfg, p); err != nil {
							fmt.Printf("[Console] Failed to start TUN: %v\n", err)
						} else {
							fmt.Println("[Console] TUN started")
						}
					}
				} else {
					fmt.Println("[Console] TUN not available on this platform")
				}
			case "4", "status":
				fmt.Printf("  Proxy: %v\n", app.IsRunning())
				fmt.Printf("  Mode: %s\n", app.GetMode())
				down, up := app.GetStats()
				fmt.Printf("  Download: %s\n", formatBytes(down))
				fmt.Printf("  Upload: %s\n", formatBytes(up))
			case "5", "reload":
				app.ReloadConfig()
				fmt.Println("[Console] Config reloaded")
			case "6", "mode":
				fmt.Printf("Current mode: %s\n", app.GetMode())
				fmt.Println("Available: mitm, transparent, tls-rf, quic")
				fmt.Print("Enter new mode (or empty to cancel): ")
				newMode, _ := reader.ReadString('\n')
				newMode = strings.TrimSpace(strings.ToLower(newMode))
				if newMode != "" {
					app.SetMode(newMode)
					fmt.Printf("[Console] Mode set to: %s\n", newMode)
				}
			case "7", "help", "?":
				fmt.Println(`
Console Commands:
  1/proxy     - Toggle proxy server
  2/sysproxy  - Toggle system proxy
  3/tun       - Toggle TUN mode
  4/status    - Show current status
  5/reload    - Reload configuration
  6/mode      - Change proxy mode
  7/help/?    - Show this help
  quit/exit   - Exit application`)
			case "quit", "exit", "q":
				fmt.Println("[Console] Shutting down...")
				app.Stop()
				if linuxFeatures != nil {
					linuxFeatures.DisableSysProxy()
					linuxFeatures.StopTUN()
				}
				apiServer.Stop()
				os.Exit(0)
			case "":
				continue
			default:
				fmt.Println("[Console] Unknown command. Type 'help' for available commands.")
			}
		}
	}()
}

func main() {
	flag.Parse()

	if showVersion {
		fmt.Printf("SniShaper CLI v%s\n", Version)
		fmt.Printf("Go %s (%s/%s)\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
		return
	}

	if showHelp {
		printBanner()
		flag.Usage()
		return
	}

	// Initialize templates
	initTemplates()

	printBanner()

	rulesPath, settingsPath, certPath := ensureConfigPaths()

	fmt.Printf("[Info] Config dir: %s\n", filepath.Dir(settingsPath))
	fmt.Printf("[Info] Rules file: %s\n", rulesPath)
	fmt.Printf("[Info] Settings file: %s\n", settingsPath)
	fmt.Printf("[Info] Cert dir: %s\n", certPath)

	actualListen := listenAddr
	if actualListen == "" {
		actualListen = "127.0.0.1:8080"
	}
	fmt.Printf("[Info] Listen address: %s\n", actualListen)

	app, err := NewCLIApp(rulesPath, settingsPath, certPath, actualListen)
	if err != nil {
		log.Fatalf("[Error] Failed to create app: %v", err)
	}

	if mode != "" {
		if err := app.SetMode(mode); err != nil {
			log.Printf("[Warn] Failed to set mode %s: %v", mode, err)
		} else {
			fmt.Printf("[Info] Proxy mode set to: %s\n", mode)
		}
	}

	fmt.Println("[Info] Starting proxy server...")
	if err := app.Start(); err != nil {
		log.Fatalf("[Error] Failed to start proxy: %v", err)
	}

	fmt.Printf("[Info] Proxy running in %s mode\n", app.GetMode())
	fmt.Println("[Info] Interactive console: type 'help' for commands, or press Ctrl+C to stop")
	fmt.Println()

	// If auto update is enabled, fetch IPs immediately
	cfg := app.ruleManager.GetCloudflareConfig()
	if cfg.AutoUpdate {
		fmt.Println("[Info] Cloudflare IP auto-update enabled, fetching initial IPs...")
		go app.RefreshCloudflareIPPool()
	}

	// Start periodic auto-update if enabled (every 24 hours)
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			config := app.ruleManager.GetCloudflareConfig()
			if config.AutoUpdate {
				log.Printf("[Cloudflare] Running scheduled auto-update")
				app.RefreshCloudflareIPPool()
			}
		}
	}()
	actualAPIAddr := apiAddr
	if actualAPIAddr == "" {
		actualAPIAddr = "0.0.0.0:5173"
	}
	fmt.Printf("[Info] API server listening on %s\n", actualAPIAddr)
	fmt.Printf("[Info] Endpoints: /status, /stats, /logs, /mode, /reload, /stop\n")
	fmt.Println()

	apiServer := NewAPIServer(actualAPIAddr, app)
	go func() {
		if err := apiServer.Start(); err != nil && err != http.ErrServerClosed {
			log.Printf("[Error] API server error: %v", err)
		}
	}()

	stats := newStatsDisplay()
	stats.start(app)
	defer stats.stop()

	startInteractiveConsole(app, apiServer, apiServer.linuxFeatures)

	waitForSignal(app)
}
