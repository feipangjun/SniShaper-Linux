package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// 应用版本号 - 发布时修改此值
const AppVersion = "1.26"

// UpdateInfo 远程更新信息
type UpdateInfo struct {
	LatestVersion string `json:"latest_version"`
	DownloadURL   string `json:"download_url"`
	ReleaseNotes  string `json:"release_notes,omitempty"`
	IsDevVersion  bool   `json:"is_dev_version,omitempty"`
}

// GetLocalVersion 获取本地版本号
func GetLocalVersion() string {
	return AppVersion
}

// ParseVersion 解析版本号为可比较的数字
// 支持格式: "1.26.0" 或 "1.26" (自动补零为 "1.26.0")
// 例如: "1.26.0" -> 1026000, "1.26" -> 1026000
func ParseVersion(version string) (int, error) {
	version = strings.TrimSpace(version)
	parts := strings.Split(version, ".")

	// 支持两位或三位版本号
	if len(parts) < 2 || len(parts) > 3 {
		return 0, fmt.Errorf("invalid version format: %s", version)
	}

	// 如果只有两位，自动补零
	if len(parts) == 2 {
		parts = append(parts, "0")
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("invalid major version: %s", parts[0])
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, fmt.Errorf("invalid minor version: %s", parts[1])
	}

	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return 0, fmt.Errorf("invalid patch version: %s", parts[2])
	}

	// 转换为整数以便比较: major * 1000000 + minor * 1000 + patch
	return major*1000000 + minor*1000 + patch, nil
}

// IsNewerVersion 检查远程版本是否比本地版本新
func IsNewerVersion(localVersion, remoteVersion string) (bool, error) {
	localNum, err := ParseVersion(localVersion)
	if err != nil {
		return false, fmt.Errorf("parse local version failed: %w", err)
	}

	remoteNum, err := ParseVersion(remoteVersion)
	if err != nil {
		return false, fmt.Errorf("parse remote version failed: %w", err)
	}

	return remoteNum > localNum, nil
}

// GetAppExecutablePath 获取应用程序可执行文件路径
func GetAppExecutablePath() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}
	return execPath, nil
}

// GetAppDirectory 获取应用程序所在目录
func GetAppDirectory() (string, error) {
	execPath, err := GetAppExecutablePath()
	if err != nil {
		return "", err
	}
	return filepath.Dir(execPath), nil
}
