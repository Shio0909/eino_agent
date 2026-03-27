// Package config 配置管理
package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config 应用配置
type Config struct {
	Server      ServerConfig      `yaml:"server"`
	Auth        AuthConfig        `yaml:"auth"`
	LLM         LLMConfig         `yaml:"llm"`
	Embedding   EmbeddingConfig   `yaml:"embedding"`
	Reranker    RerankerConfig    `yaml:"reranker"`
	Database    DatabaseConfig    `yaml:"database"`
	RAG         RAGConfig         `yaml:"rag"`
	Agent       AgentConfig       `yaml:"agent"`
	Security    SecurityConfig    `yaml:"security"`
	Memory      MemoryConfig      `yaml:"memory"`
	DocReader   DocReaderConfig   `yaml:"docreader"`
	Redis       RedisConfig       `yaml:"redis"`
	ImportQueue ImportQueueConfig `yaml:"import_queue"`
	MCP         MCPConfig         `yaml:"mcp"`
	GraphRAG    GraphRAGConfig    `yaml:"graphrag"`
}

// SecurityConfig 安全策略配置。
type SecurityConfig struct {
	PromptGuard PromptGuardConfig `yaml:"prompt_guard"`
	URLPolicy   URLPolicyConfig   `yaml:"url_policy"`
}

// URLPolicyConfig URL 导入安全策略。
type URLPolicyConfig struct {
	AllowPrivateNetworks bool     `yaml:"allow_private_networks"`
	AllowedSchemes       []string `yaml:"allowed_schemes"`
	BlockedHosts         []string `yaml:"blocked_hosts"`
	AllowedDomains       []string `yaml:"allowed_domains"`
	BlockedDomains       []string `yaml:"blocked_domains"`
	MaxRedirects         int      `yaml:"max_redirects"`
}

// PromptGuardConfig Prompt 注入/越权调用防护配置。
type PromptGuardConfig struct {
	Enabled               *bool    `yaml:"enabled"`
	BlockOnHigh           *bool    `yaml:"block_on_high"`
	DowngradeOnMedium     *bool    `yaml:"downgrade_on_medium"`
	ForceCitationOnMedium *bool    `yaml:"force_citation_on_medium"`
	HighRiskPatterns      []string `yaml:"high_risk_patterns"`
	MediumRiskPatterns    []string `yaml:"medium_risk_patterns"`
}

// ImportQueueConfig 异步导入队列配置
type ImportQueueConfig struct {
	Enabled         bool   `yaml:"enabled"`
	Provider        string `yaml:"provider"`
	URL             string `yaml:"url"`
	QueueName       string `yaml:"queue_name"`
	ConsumerTag     string `yaml:"consumer_tag"`
	TempDir         string `yaml:"temp_dir"`
	PrefetchCount   int    `yaml:"prefetch_count"`
	StateTTLMinutes int    `yaml:"state_ttl_minutes"`
}

// MemoryConfig 记忆配置
type MemoryConfig struct {
	Enabled                    bool `yaml:"enabled"`
	WindowSize                 int  `yaml:"window_size"`
	ShortTermCacheTTLMinutes   int  `yaml:"short_term_cache_ttl_minutes"`
	EnableLongTerm             bool `yaml:"enable_long_term"`
	LongTermSessionLimit       int  `yaml:"long_term_session_limit"`
	LongTermMessagesPerSession int  `yaml:"long_term_messages_per_session"`
	MaxContextChars            int  `yaml:"max_context_chars"`
}

// AuthConfig 鉴权配置
type AuthConfig struct {
	Enabled                  bool   `yaml:"enabled"`
	JWTSecret                string `yaml:"jwt_secret"`
	AccessTokenExpireMinutes int    `yaml:"access_token_expire_minutes"`
	AdminTenantID            int    `yaml:"admin_tenant_id"`
	AdminUsername            string `yaml:"admin_username"`
	AdminPassword            string `yaml:"admin_password"`
	UserTenantID             int    `yaml:"user_tenant_id"`
	UserUsername             string `yaml:"user_username"`
	UserPassword             string `yaml:"user_password"`
}

// GraphRAGConfig GraphRAG 图谱增强检索配置
type GraphRAGConfig struct {
	Enabled            bool       `yaml:"enabled"`             // 是否启用 GraphRAG
	Neo4jURI           string     `yaml:"neo4j_uri"`           // Neo4j Bolt URI
	Neo4jUsername      string     `yaml:"neo4j_username"`      // Neo4j 用户名
	Neo4jPassword      string     `yaml:"neo4j_password"`      // Neo4j 密码
	ExtractTemperature float64    `yaml:"extract_temperature"` // 实体抽取温度（建议 0.1）
	TopK               int        `yaml:"top_k"`               // 图谱检索返回 chunk 数
	LightLLM           *LLMConfig `yaml:"light_llm"`           // 查询时实体抽取用的轻量模型（可选）
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Host        string   `yaml:"host"`
	Port        int      `yaml:"port"`
	Mode        string   `yaml:"mode"` // debug, release
	CORSOrigins []string `yaml:"cors_origins"`
}

// LLMConfig LLM 配置
type LLMConfig struct {
	Provider    string  `yaml:"provider"` // openai, azure, ollama
	BaseURL     string  `yaml:"base_url"`
	APIKey      string  `yaml:"api_key"`
	ModelID     string  `yaml:"model_id"`
	Temperature float64 `yaml:"temperature"`
	MaxTokens   int     `yaml:"max_tokens"`
}

// EmbeddingConfig Embedding 配置
type EmbeddingConfig struct {
	Provider   string `yaml:"provider"` // openai, jina, local
	BaseURL    string `yaml:"base_url"`
	APIKey     string `yaml:"api_key"`
	ModelID    string `yaml:"model_id"`
	Dimensions int    `yaml:"dimensions"`
}

// RerankerConfig Reranker 配置
type RerankerConfig struct {
	Enabled   bool    `yaml:"enabled"`
	Provider  string  `yaml:"provider"` // jina, cohere, local
	BaseURL   string  `yaml:"base_url"`
	APIKey    string  `yaml:"api_key"`
	ModelID   string  `yaml:"model_id"`
	TopK      int     `yaml:"top_k"`
	Threshold float64 `yaml:"threshold"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	// PostgreSQL + pgvector
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
	SSLMode  string `yaml:"sslmode"`

	// Milvus (可选)
	MilvusAddr string `yaml:"milvus_addr"`
}

// RAGConfig RAG 配置
type RAGConfig struct {
	// 检索配置
	TopK                     int  `yaml:"top_k"`
	EnableHybrid             bool `yaml:"enable_hybrid"`  // 混合检索
	EnableRewrite            bool `yaml:"enable_rewrite"` // 查询重写
	EnableRerank             bool `yaml:"enable_rerank"`  // 重排序
	EmbeddingCacheTTLMinutes int  `yaml:"embedding_cache_ttl_minutes"`
	RetrievalCacheTTLMinutes int  `yaml:"retrieval_cache_ttl_minutes"`

	// 分块配置
	ChunkSize     int    `yaml:"chunk_size"`
	ChunkOverlap  int    `yaml:"chunk_overlap"`
	ChunkStrategy string `yaml:"chunk_strategy"` // recursive, markdown, auto

	// 文档路径
	DocumentsPath string `yaml:"documents_path"`
}

// AgentConfig Agent 配置
type AgentConfig struct {
	Enabled      bool   `yaml:"enabled"`
	SystemPrompt string `yaml:"system_prompt"`
	MaxSteps     int    `yaml:"max_steps"`

	// 工具配置
	EnableKnowledgeTool bool `yaml:"enable_knowledge_tool"`
	EnableWebSearch     bool `yaml:"enable_web_search"`

	// Skills 配置（Eino 原生渐进式披露）
	EnableSkills bool   `yaml:"enable_skills"`
	SkillsDir    string `yaml:"skills_dir"`

	// Web 搜索配置
	TavilyAPIKey string `yaml:"tavily_api_key"`
	SerpAPIKey   string `yaml:"serp_api_key"`

	// Agentic RAG 配置
	AgenticRAG AgenticRAGConfig `yaml:"agentic_rag"`
}

// AgenticRAGConfig Agentic RAG 配置（含 Query Router / Decomposition / Knowledge Refinement / Self-Reflection）
type AgenticRAGConfig struct {
	Enabled           bool    `yaml:"enabled"`             // 是否启用 Agentic RAG
	MaxRetries        int     `yaml:"max_retries"`         // 最大重试次数（防死循环）
	QualityThreshold  float64 `yaml:"quality_threshold"`   // 检索质量阈值 (0-1)
	EnableWebFallback bool    `yaml:"enable_web_fallback"` // 重试失败后是否降级到 Web 搜索
	MaxRunSteps       int     `yaml:"max_run_steps"`       // Graph 最大运行步数

	// 轻量模型配置：用于 classify / refine 等不需要强推理的节点，降低延迟
	LightLLM *LLMConfig `yaml:"light_llm,omitempty"`
}

// DocReaderConfig DocReader 文档解析服务配置
type DocReaderConfig struct {
	Enabled     bool   `yaml:"enabled"`
	Mode        string `yaml:"mode"`          // local, grpc, auto, mineru, mineru_with_fallback
	Endpoint    string `yaml:"endpoint"`      // gRPC 地址，如 localhost:50051
	MaxFileSize int64  `yaml:"max_file_size"` // 最大文件大小 (MB)

	// MinerU HTTP 服务端点（mode=mineru 或 mineru_with_fallback 时使用）
	MinerUEndpoint string `yaml:"mineru_endpoint"` // 如 http://mineru:8000

	// MinIO 配置 (用于存储解析后的图片)
	MinIOEndpoint  string `yaml:"minio_endpoint"`
	MinIOAccessKey string `yaml:"minio_access_key"`
	MinIOSecretKey string `yaml:"minio_secret_key"`
	MinIOBucket    string `yaml:"minio_bucket"`

	// VLM 配置 (多模态)
	EnableMultimodal bool   `yaml:"enable_multimodal"`
	VLMBaseURL       string `yaml:"vlm_base_url"`
	VLMAPIKey        string `yaml:"vlm_api_key"`
	VLMModel         string `yaml:"vlm_model"`

	// Web URL 解析配置
	UserAgent             string                    `yaml:"user_agent"`
	RequestTimeoutSeconds int                       `yaml:"request_timeout_seconds"`
	MaxDownloadBytes      int64                     `yaml:"max_download_bytes"`
	RenderMode            string                    `yaml:"render_mode"` // disabled, auto, always
	Playwright            DocReaderPlaywrightConfig `yaml:"playwright"`
}

// DocReaderPlaywrightConfig Playwright 浏览器渲染配置。
type DocReaderPlaywrightConfig struct {
	Enabled        bool     `yaml:"enabled"`
	Command        string   `yaml:"command"`
	Args           []string `yaml:"args"`
	TimeoutSeconds int      `yaml:"timeout_seconds"`
	WaitUntil      string   `yaml:"wait_until"`
	MaxHTMLBytes   int64    `yaml:"max_html_bytes"`
}

// RedisConfig Redis 配置
type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

// MCPConfig MCP 配置
type MCPConfig struct {
	Enabled bool              `yaml:"enabled"`
	Servers []MCPServerConfig `yaml:"servers"`
}

// MCPServerConfig 单个 MCP 服务器配置
type MCPServerConfig struct {
	Name         string   `yaml:"name"`           // 服务器名称（用于日志）
	Endpoint     string   `yaml:"endpoint"`       // MCP 端点 URL，如 http://localhost:3001/sse 或 https://mcp.tavily.com/mcp/
	Transport    string   `yaml:"transport"`      // 传输协议: sse / streamable_http / stdio
	Command      string   `yaml:"command"`        // stdio 模式下启动命令，如 docker
	Args         []string `yaml:"args"`           // stdio 模式下启动参数
	Env          []string `yaml:"env"`            // stdio 模式下注入的环境变量，格式 KEY=VALUE
	APIKey       string   `yaml:"api_key"`        // 可选: 通过 Header 传递的 API Key
	APIKeyHeader string   `yaml:"api_key_header"` // 可选: API Key 的 Header 名称，默认 Authorization
	APIKeyPrefix string   `yaml:"api_key_prefix"` // 可选: API Key 前缀，默认 Bearer
	ToolNames    []string `yaml:"tool_names"`     // 只获取指定工具，为空则获取全部
}

// expandEnvWithDefault 支持 ${VAR:-default} 语法的环境变量替换
func expandEnvWithDefault(s string) string {
	// 匹配 ${VAR:-default} 或 ${VAR}
	re := regexp.MustCompile(`\$\{([^}:]+)(:-([^}]*))?\}`)
	return re.ReplaceAllStringFunc(s, func(match string) string {
		// 解析变量名和默认值
		parts := re.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		varName := parts[1]
		defaultVal := ""
		if len(parts) >= 4 {
			defaultVal = parts[3]
		}

		// 获取环境变量，如果不存在则使用默认值
		if val := os.Getenv(varName); val != "" {
			return val
		}
		return defaultVal
	})
}

// Load 加载配置文件
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	// 环境变量替换（支持 ${VAR:-default} 语法）
	content := expandEnvWithDefault(string(data))

	// 处理空字符串导致的 YAML 解析问题
	content = strings.ReplaceAll(content, `: ""`, `: ""`)

	var cfg Config
	if err := yaml.Unmarshal([]byte(content), &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// 设置默认值
	setDefaults(&cfg)

	return &cfg, nil
}

// Save 保存配置到文件（YAML）
func Save(path string, cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	return nil
}

// setDefaults 设置默认值
func setDefaults(cfg *Config) {
	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Server.Mode == "" {
		cfg.Server.Mode = "debug"
	}

	if cfg.Auth.JWTSecret == "" {
		cfg.Auth.JWTSecret = "change-me-in-production"
	}
	if cfg.Auth.AccessTokenExpireMinutes == 0 {
		cfg.Auth.AccessTokenExpireMinutes = 120
	}
	if cfg.Auth.AdminTenantID == 0 {
		cfg.Auth.AdminTenantID = 1
	}
	if cfg.Auth.AdminUsername == "" {
		cfg.Auth.AdminUsername = "admin"
	}
	if cfg.Auth.AdminPassword == "" {
		cfg.Auth.AdminPassword = "admin123"
	}
	if cfg.Auth.UserTenantID == 0 {
		cfg.Auth.UserTenantID = cfg.Auth.AdminTenantID
	}
	if cfg.Auth.UserUsername == "" {
		cfg.Auth.UserUsername = "user"
	}
	if cfg.Auth.UserPassword == "" {
		cfg.Auth.UserPassword = "user123"
	}

	if cfg.LLM.Temperature == 0 {
		cfg.LLM.Temperature = 0.7
	}
	if cfg.LLM.MaxTokens == 0 {
		cfg.LLM.MaxTokens = 4096
	}

	if cfg.Embedding.Dimensions == 0 {
		cfg.Embedding.Dimensions = 1536
	}

	if cfg.RAG.TopK == 0 {
		cfg.RAG.TopK = 10
	}
	if cfg.RAG.EmbeddingCacheTTLMinutes == 0 {
		cfg.RAG.EmbeddingCacheTTLMinutes = 1440
	}
	if cfg.RAG.RetrievalCacheTTLMinutes == 0 {
		cfg.RAG.RetrievalCacheTTLMinutes = 10
	}
	if cfg.RAG.ChunkSize == 0 {
		cfg.RAG.ChunkSize = 512
	}
	if cfg.RAG.ChunkOverlap == 0 {
		cfg.RAG.ChunkOverlap = 50
	}
	if cfg.RAG.ChunkStrategy == "" {
		cfg.RAG.ChunkStrategy = "auto"
	}

	if cfg.Agent.MaxSteps == 0 {
		cfg.Agent.MaxSteps = 10
	}

	if cfg.Security.PromptGuard.Enabled == nil {
		v := true
		cfg.Security.PromptGuard.Enabled = &v
	}
	if cfg.Security.PromptGuard.BlockOnHigh == nil {
		v := true
		cfg.Security.PromptGuard.BlockOnHigh = &v
	}
	if cfg.Security.PromptGuard.DowngradeOnMedium == nil {
		v := true
		cfg.Security.PromptGuard.DowngradeOnMedium = &v
	}
	if cfg.Security.PromptGuard.ForceCitationOnMedium == nil {
		v := true
		cfg.Security.PromptGuard.ForceCitationOnMedium = &v
	}
	if len(cfg.Security.URLPolicy.AllowedSchemes) == 0 {
		cfg.Security.URLPolicy.AllowedSchemes = []string{"http", "https"}
	}
	if len(cfg.Security.URLPolicy.BlockedHosts) == 0 {
		cfg.Security.URLPolicy.BlockedHosts = []string{"localhost", "127.0.0.1", "::1"}
	}
	if cfg.Security.URLPolicy.MaxRedirects == 0 {
		cfg.Security.URLPolicy.MaxRedirects = 5
	}

	// Memory 默认值
	if cfg.Memory.WindowSize == 0 {
		cfg.Memory.WindowSize = 8
	}
	if cfg.Memory.ShortTermCacheTTLMinutes == 0 {
		cfg.Memory.ShortTermCacheTTLMinutes = 60
	}
	if cfg.Memory.LongTermSessionLimit == 0 {
		cfg.Memory.LongTermSessionLimit = 5
	}
	if cfg.Memory.LongTermMessagesPerSession == 0 {
		cfg.Memory.LongTermMessagesPerSession = 2
	}
	if cfg.Memory.MaxContextChars == 0 {
		cfg.Memory.MaxContextChars = 3000
	}

	// Agentic RAG 默认值
	if cfg.Agent.AgenticRAG.MaxRetries == 0 {
		cfg.Agent.AgenticRAG.MaxRetries = 3
	}
	if cfg.Agent.AgenticRAG.QualityThreshold == 0 {
		cfg.Agent.AgenticRAG.QualityThreshold = 0.6
	}
	if cfg.Agent.AgenticRAG.MaxRunSteps == 0 {
		cfg.Agent.AgenticRAG.MaxRunSteps = 20
	}

	// Redis 默认值
	if cfg.DocReader.Mode == "" {
		cfg.DocReader.Mode = "auto"
	}
	if cfg.DocReader.RequestTimeoutSeconds == 0 {
		cfg.DocReader.RequestTimeoutSeconds = 60
	}
	if cfg.DocReader.MaxDownloadBytes == 0 {
		cfg.DocReader.MaxDownloadBytes = 10 << 20
	}
	if cfg.DocReader.UserAgent == "" {
		cfg.DocReader.UserAgent = "Mozilla/5.0 (compatible; EinoAgent/1.0)"
	}
	if cfg.DocReader.RenderMode == "" {
		cfg.DocReader.RenderMode = "auto"
	}
	if cfg.DocReader.Playwright.TimeoutSeconds == 0 {
		cfg.DocReader.Playwright.TimeoutSeconds = 90
	}
	if cfg.DocReader.Playwright.WaitUntil == "" {
		cfg.DocReader.Playwright.WaitUntil = "networkidle"
	}
	if cfg.DocReader.Playwright.MaxHTMLBytes == 0 {
		cfg.DocReader.Playwright.MaxHTMLBytes = 2 << 20
	}
	if cfg.DocReader.Playwright.Command == "" {
		cfg.DocReader.Playwright.Command = "node"
	}
	if len(cfg.DocReader.Playwright.Args) == 0 {
		cfg.DocReader.Playwright.Args = []string{"scripts/playwright-docreader.js"}
	}

	// Redis 默认值
	if cfg.Redis.Addr == "" {
		cfg.Redis.Addr = "localhost:6379"
	}

	if cfg.ImportQueue.Provider == "" {
		cfg.ImportQueue.Provider = "rabbitmq"
	}
	if cfg.ImportQueue.URL == "" {
		cfg.ImportQueue.URL = "amqp://guest:guest@localhost:5672/"
	}
	if cfg.ImportQueue.QueueName == "" {
		cfg.ImportQueue.QueueName = "knowledge_imports"
	}
	if cfg.ImportQueue.ConsumerTag == "" {
		cfg.ImportQueue.ConsumerTag = "eino-agent-import-worker"
	}
	if cfg.ImportQueue.PrefetchCount == 0 {
		cfg.ImportQueue.PrefetchCount = 1
	}

	// GraphRAG 默认值
	if cfg.GraphRAG.Neo4jURI == "" {
		cfg.GraphRAG.Neo4jURI = "bolt://localhost:7687"
	}
	if cfg.GraphRAG.Neo4jUsername == "" {
		cfg.GraphRAG.Neo4jUsername = "neo4j"
	}
	if cfg.GraphRAG.ExtractTemperature == 0 {
		cfg.GraphRAG.ExtractTemperature = 0.1
	}
	if cfg.GraphRAG.TopK == 0 {
		cfg.GraphRAG.TopK = 10
	}
}
