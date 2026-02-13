package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const WorkspaceDir = "workspace"

type Config struct {
	Builds    []StageConfig           `yaml:"builds"`            // 构建阶段配置
	Deploys   []DeploymentStep        `yaml:"deploys"`           // 部署步骤配置
	Container ContainerConfig         `yaml:"container"`         // 容器配置
	Servers   map[string]ServerConfig `yaml:"servers"`           // 服务器配置
	Clone     *CloneConfig            `yaml:"clone,omitempty"`   // Git克隆配置
	Cleanup   *CleanupConfig          `yaml:"cleanup,omitempty"` // 清理配置
}

type CloneConfig struct {
	URL    string `yaml:"url"`    // Git仓库URL
	Branch string `yaml:"branch"` // Git分支
}

type ContainerConfig struct {
	Type string `yaml:"type"` // 容器类型：docker / podman
}

type ServerConfig struct {
	Host     string `yaml:"host"`     // 服务器地址
	User     string `yaml:"user"`     // SSH用户名
	Port     int    `yaml:"port"`     // SSH端口
	KeyPath  string `yaml:"key_path"` // SSH私钥路径
	Password string `yaml:"password"` // SSH密码（可选）
}

type StageConfig struct {
	Name            string                `yaml:"name"`              // 阶段名称
	Image           string                `yaml:"image"`             // 容器镜像
	WorkingDir      string                `yaml:"working_dir"`       // 容器内工作目录
	Environment     []string              `yaml:"environment"`       // 环境变量
	CopyToContainer []CopyToContainerPath `yaml:"copy_to_container"` // 复制到容器的文件
	CopyToLocal     []CopyToLocalPath     `yaml:"copy_to_local"`     // 复制到本地的文件
	Commands        []string              `yaml:"commands"`          // 执行命令
}

type CopyToContainerPath struct {
	From  string `yaml:"from"`   // 本地源路径（相对于项目目录）
	ToDir string `yaml:"to_dir"` // 容器内目标目录
}

type CopyToLocalPath struct {
	From       string `yaml:"from"`         // 容器内源路径
	ToDir      string `yaml:"to_dir"`       // 本地目标目录（相对于项目目录）
	EmptyToDir bool   `yaml:"empty_to_dir"` // 是否在复制前清空目标目录
}

type DeploymentStep struct {
	Name     string   `yaml:"name"`     // 步骤名称
	Server   string   `yaml:"server"`   // 引用的服务器名称
	Commands []string `yaml:"commands"` // 远程执行命令
	From     string   `yaml:"from"`     // 本地源路径（相对于项目目录）
	To       string   `yaml:"to"`       // 远程目标路径，支持绝对路径和相对路径
	// 绝对路径：如 /opt/myapp/
	// 相对路径：如 ./myapp/（相对于用户 home 目录）
	// 注意：不支持 ~/myapp/ 写法，会在远程创建名为 "~/myapp/" 的目录
}

type ConfigInfo struct {
	DirName  string // 项目目录名称
	FilePath string // 配置文件路径
}

func LoadConfigInfo(workspaceDir string) ([]ConfigInfo, error) {
	entries, err := os.ReadDir(workspaceDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read workspace directory: %w", err)
	}

	var configs []ConfigInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirName := entry.Name()
		configPath := filepath.Join(workspaceDir, dirName, "config.yaml")

		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			continue
		}

		configs = append(configs, ConfigInfo{
			DirName:  dirName,
			FilePath: configPath,
		})
	}

	return configs, nil
}

func Load(configPath string) (*Config, string, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, "", fmt.Errorf("failed to parse config file: %w", err)
	}

	basicPath := filepath.Dir(configPath)

	applyEnvSubstitution(&cfg)

	return &cfg, basicPath, nil
}

func applyEnvSubstitution(cfg *Config) {
	for i := range cfg.Builds {
		for j := range cfg.Builds[i].Commands {
			cfg.Builds[i].Commands[j] = os.ExpandEnv(cfg.Builds[i].Commands[j])
		}
		for j := range cfg.Builds[i].Environment {
			cfg.Builds[i].Environment[j] = os.ExpandEnv(cfg.Builds[i].Environment[j])
		}
	}

	for i := range cfg.Deploys {
		for j := range cfg.Deploys[i].Commands {
			cfg.Deploys[i].Commands[j] = os.ExpandEnv(cfg.Deploys[i].Commands[j])
		}
	}
}

func FindStage(stages []StageConfig, name string) *StageConfig {
	for i := range stages {
		if stages[i].Name == name {
			return &stages[i]
		}
	}
	return nil
}

func FindDeploymentStep(steps []DeploymentStep, name string) *DeploymentStep {
	for i := range steps {
		if steps[i].Name == name {
			return &steps[i]
		}
	}
	return nil
}

func GetServer(cfg *Config, name string) *ServerConfig {
	if server, ok := cfg.Servers[name]; ok {
		return &server
	}
	return nil
}

type CleanupConfig struct {
	Enable bool     `yaml:"enable"` // 是否执行清理，设为 true 会删除 source 目录
	Dirs   []string `yaml:"dirs"`   // 额外清理的目录，相对于 workspace/<project>/
}
