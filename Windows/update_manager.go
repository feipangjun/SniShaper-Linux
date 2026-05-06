package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// UpdateManager 更新管理器
type UpdateManager struct {
	updateURL   string
	versionFile string
	timeout     time.Duration
}

// NewUpdateManager 创建新的更新管理器
func NewUpdateManager(updateURL, versionFile string) *UpdateManager {
	return &UpdateManager{
		updateURL:   updateURL,
		versionFile: versionFile,
		timeout:     30 * time.Second,
	}
}

// CheckForUpdates 检查更新
func (um *UpdateManager) CheckForUpdates(ctx context.Context, tempDir string) (*UpdateInfo, error) {
	// 创建带超时的HTTP客户端
	client := &http.Client{
		Timeout: um.timeout,
	}

	// 请求远程版本信息文件
	req, err := http.NewRequestWithContext(ctx, "GET", um.versionFile, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 添加请求头以避免被服务器拒绝
	req.Header.Set("User-Agent", "SniShaper-UpdateChecker/"+GetLocalVersion())
	req.Header.Set("Accept", "text/plain,application/json,*/*")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("无法连接到更新服务器：网络请求失败 - %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("更新服务器返回错误：HTTP %d", resp.StatusCode)
	}

	// 读取响应内容
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取版本信息失败：- %v", err)
	}

	// 将 update.txt 保存到 temp 目录
	if tempDir != "" {
		updateFilePath := filepath.Join(tempDir, "update.txt")
		if err := os.MkdirAll(tempDir, 0755); err != nil {
			log.Printf("[warn] Failed to create temp dir for update.txt: %v", err)
		} else if err := os.WriteFile(updateFilePath, body, 0644); err != nil {
			log.Printf("[warn] Failed to save update.txt to temp: %v", err)
		} else {
			log.Printf("[info] update.txt saved to: %s", updateFilePath)
		}
	}

	content := strings.TrimSpace(string(body))
	lines := strings.Split(content, "\n")
	if len(lines) < 2 {
		return nil, fmt.Errorf("版本信息文件格式错误：行数不足，至少需要版本号和下载链接")
	}

	latestVersion := strings.TrimSpace(lines[0])
	downloadURL := strings.TrimSpace(lines[1])

	// 验证版本号格式
	if _, err := ParseVersion(latestVersion); err != nil {
		return nil, fmt.Errorf("版本号格式非法，应为 x.y.z 格式：- %v", err)
	}

	// 验证下载链接格式
	if _, err := url.ParseRequestURI(downloadURL); err != nil {
		return nil, fmt.Errorf("下载链接格式无效：- %v", err)
	}

	// 比对本地版本与远程版本
	localVersion := GetLocalVersion()
	isNewer, err := IsNewerVersion(localVersion, latestVersion)
	if err != nil {
		return nil, fmt.Errorf("版本比对失败：- %v", err)
	}

	// 如果本地版本不低于远程版本，返回开发版本标识
	if !isNewer {
		return &UpdateInfo{
			LatestVersion: latestVersion,
			DownloadURL:   downloadURL,
			IsDevVersion:  true,
		}, nil
	}

	// 解析可选的更新说明（第三行）
	releaseNotes := ""
	if len(lines) >= 3 {
		releaseNotes = strings.TrimSpace(lines[2])
	}

	return &UpdateInfo{
		LatestVersion: latestVersion,
		DownloadURL:   downloadURL,
		ReleaseNotes:  releaseNotes,
		IsDevVersion:  false,
	}, nil
}
