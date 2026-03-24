// Package prompt Prompt 模板管理
//
// 【Eino 特点】参考 WeKnora 的 prompt 模板机制
// 提供模板加载、变量替换、动态构建功能
package prompt

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	einoprompt "github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/schema"
	"gopkg.in/yaml.v3"
)

// Template Prompt 模板
type Template struct {
	ID               string `yaml:"id"`
	Name             string `yaml:"name"`
	Description      string `yaml:"description"`
	HasKnowledgeBase bool   `yaml:"has_knowledge_base"`
	HasWebSearch     bool   `yaml:"has_web_search"`
	Content          string `yaml:"content"`
}

// TemplateFile 模板文件结构
type TemplateFile struct {
	Templates []Template `yaml:"templates"`
}

// Variables 模板变量
type Variables struct {
	Query         string                 // 用户查询
	Contexts      string                 // 检索到的上下文
	CurrentTime   string                 // 当前时间
	CurrentWeek   string                 // 当前星期
	KnowledgeBases []KnowledgeBaseInfo   // 知识库信息
	CustomVars    map[string]interface{} // 自定义变量
}

// KnowledgeBaseInfo 知识库信息
type KnowledgeBaseInfo struct {
	ID          string
	Name        string
	Type        string
	Description string
	DocCount    int
}

// Manager 模板管理器
type Manager struct {
	mu            sync.RWMutex
	templates     map[string]*Template
	systemPrompts map[string]*Template
	contextTpls   map[string]*Template
	rewriteTpls   map[string]*Template
}

// NewManager 创建模板管理器
func NewManager() *Manager {
	m := &Manager{
		templates:     make(map[string]*Template),
		systemPrompts: make(map[string]*Template),
		contextTpls:   make(map[string]*Template),
		rewriteTpls:   make(map[string]*Template),
	}

	// 注册默认模板
	m.registerDefaults()

	return m
}

// registerDefaults 注册默认模板
func (m *Manager) registerDefaults() {
	// 默认系统提示词
	m.systemPrompts["default"] = &Template{
		ID:   "default",
		Name: "默认系统提示词",
		Content: `你是一个专业的知识库问答助手。

## 忠实性原则（最高优先级）
- 你的回答**必须且只能**基于提供的参考资料，**严禁**使用自身训练知识补充或推断
- 如果参考资料不足以回答问题，请直接回复：「根据现有资料，暂无法提供该问题的确切答案。建议补充相关文档后重试。」
- 禁止编造文档名称、命令、版本号、配置项等任何具体信息
- 允许对资料内容进行总结和归纳，但不得添加资料中没有的事实

## 回答规范
1. 回答准确、简洁、条理清晰
2. 引用资料时使用 [来源X] 标注
3. 综合多个来源时说明各来源

当前时间：{{.CurrentTime}}`,
	}

	// RAG 系统提示词
	m.systemPrompts["rag"] = &Template{
		ID:               "rag",
		Name:             "RAG 问答模式",
		HasKnowledgeBase: true,
		Content: `你是一个专业的知识库问答助手。请严格依据检索到的参考资料回答用户问题。

## 忠实性原则（最高优先级）
- 你的回答**必须且只能**基于提供的参考资料，**严禁**使用自身训练知识补充或推断
- 如果参考资料不足以回答问题，请直接回复：「根据现有资料，暂无法提供该问题的确切答案。建议补充相关文档后重试。」
- 禁止编造文档名称、命令、版本号、配置项等任何具体信息
- 允许对资料内容进行总结和归纳，但不得添加资料中没有的事实

## 回答规范
1. **准确性**：仅基于参考资料回答
2. **完整性**：尽可能全面回答，不遗漏关键信息
3. **清晰性**：条理清晰，重点突出

## 引用格式
- 引用资料时使用 [来源X] 标注
- 综合多个来源时说明各来源

当前时间：{{.CurrentTime}}`,
	}

	// Pipeline 系统提示词
	m.systemPrompts["pipeline"] = &Template{
		ID:               "pipeline",
		Name:             "Pipeline 模式",
		HasKnowledgeBase: true,
		Content: `你是一个专业的知识库问答助手。请严格依据检索到的参考资料回答用户问题。

## 忠实性原则（最高优先级）
- 你的回答**必须且只能**基于参考资料，**严禁**使用自身训练知识补充或推断
- 如果参考资料不足以回答问题，请直接回复：「根据现有资料，暂无法提供该问题的确切答案。建议补充相关文档后重试。」
- 禁止编造文档名称、命令、版本号、配置项等任何具体信息

## 回答规范
1. 回答要准确、简洁、结构清晰
2. 关键结论附带来源标注（[来源X]）

当前时间：{{.CurrentTime}}`,
	}

	// Agent 系统提示词
	m.systemPrompts["agent"] = &Template{
		ID:               "agent",
		Name:             "Agent 模式",
		HasKnowledgeBase: true,
		HasWebSearch:     true,
		Content: `你是一个具备工具使用能力的知识库问答助手。

## 核心规则
1. 优先且通常只使用一次 knowledge_search 检索知识库，再回答
2. 回答**必须且只能**依据检索到的资料，**严禁**使用自身训练知识补充；资料不足时回复「根据现有资料，暂无法提供确切答案」
3. 默认不做无依据猜测，不使用与问题无关的工具
4. 严禁为了“继续思考”而重复调用相同或近似查询

## 检索策略
- 先把用户问题压缩为适合检索的关键词或短句，再调用 knowledge_search
- 如果第一次结果已经足够回答，就立即作答，不要继续搜索
- 只有在第一次结果明显缺少关键实体、版本、命令或定义时，才允许进行第二次更具体的检索
- 若启用了网络搜索，也仅在知识库无法覆盖且问题需要最新信息时才使用

## 停止条件
- 一旦拿到足够证据，立刻输出最终答案
- 最多进行 2 次知识库检索
- 若两次检索后仍证据不足，直接说明信息不足并结束，不要循环

## 回答要求
- 先给出简洁结论，再给出依据
- 优先引用检索结果中的原始术语、命令、字段名
- 严禁编造文档名称、命令、版本号、配置项等任何具体信息
- 严禁使用训练知识补充检索资料中没有的内容

当前时间：{{.CurrentTime}}`,
	}

	// Agentic RAG 系统提示词
	m.systemPrompts["agentic_rag"] = &Template{
		ID:               "agentic_rag",
		Name:             "Agentic RAG 模式",
		HasKnowledgeBase: true,
		Content: `你是一个会先检索、再评估质量、必要时改写查询后继续检索的知识库助手。

## 忠实性原则（最高优先级）
- 最终回答**必须且只能**基于参考资料，**严禁**使用自身训练知识补充
- 若检索到的资料不足以回答，请直接回复：「根据现有资料，暂无法提供该问题的确切答案。建议补充相关文档后重试。」
- 禁止编造文档名称、命令、版本号、配置项等任何具体信息

## 回答规范
1. 优先给出结论，再给出支撑依据
2. 使用 [来源X] 标注引用

当前时间：{{.CurrentTime}}`,
	}

	// 默认上下文模板
	m.contextTpls["default"] = &Template{
		ID:   "default",
		Name: "默认上下文模板",
		Content: `## 参考资料
{{.Contexts}}

## 用户问题
{{.Query}}

请严格基于上述参考资料回答问题。回答中请用 [来源X] 标注出处。如果资料不足以回答，请直接说明信息不足，不要自行补充。`,
	}

	// 查询重写模板
	m.rewriteTpls["default"] = &Template{
		ID:   "default",
		Name: "默认查询重写",
		Content: `请将以下用户查询重写为更适合检索的形式：

原始查询：{{.Query}}

要求：
1. 保持原意，提取关键词
2. 去除口语化表达
3. 补充可能的同义词

重写后的查询：`,
	}

	// HyDE 模板
	m.rewriteTpls["hyde"] = &Template{
		ID:   "hyde",
		Name: "HyDE 假设文档",
		Content: `请根据以下问题，生成一个可能包含答案的假设文档段落：

问题：{{.Query}}

请生成一段 100-200 字的文档内容，包含可能回答这个问题的信息：`,
	}
}

// LoadFromDir 从目录加载模板
func (m *Manager) LoadFromDir(dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return err
	}

	for _, file := range files {
		if err := m.loadFile(file); err != nil {
			return fmt.Errorf("加载模板文件 %s 失败: %w", file, err)
		}
	}

	return nil
}

// loadFile 加载单个模板文件
func (m *Manager) loadFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var tf TemplateFile
	if err := yaml.Unmarshal(data, &tf); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	filename := filepath.Base(path)
	for _, tpl := range tf.Templates {
		t := tpl // 复制
		m.templates[tpl.ID] = &t

		// 根据文件名分类
		switch {
		case strings.Contains(filename, "system"):
			m.systemPrompts[tpl.ID] = &t
		case strings.Contains(filename, "context"):
			m.contextTpls[tpl.ID] = &t
		case strings.Contains(filename, "rewrite"):
			m.rewriteTpls[tpl.ID] = &t
		}
	}

	return nil
}

// GetSystemPrompt 获取系统提示词
func (m *Manager) GetSystemPrompt(id string) (*Template, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.systemPrompts[id]
	return t, ok
}

// GetContextTemplate 获取上下文模板
func (m *Manager) GetContextTemplate(id string) (*Template, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.contextTpls[id]
	return t, ok
}

// GetRewriteTemplate 获取重写模板
func (m *Manager) GetRewriteTemplate(id string) (*Template, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.rewriteTpls[id]
	return t, ok
}

// Render 渲染模板
func (m *Manager) Render(tpl *Template, vars *Variables) (string, error) {
	if tpl == nil {
		return "", fmt.Errorf("模板不能为空")
	}

	if vars == nil {
		vars = &Variables{}
	}

	// 填充默认时间变量
	if vars.CurrentTime == "" {
		vars.CurrentTime = time.Now().Format("2006-01-02 15:04:05")
	}
	if vars.CurrentWeek == "" {
		weekdays := []string{"日", "一", "二", "三", "四", "五", "六"}
		vars.CurrentWeek = "星期" + weekdays[time.Now().Weekday()]
	}

	chatTemplate := einoprompt.FromMessages(
		schema.GoTemplate,
		schema.UserMessage(tpl.Content),
	)

	messages, err := chatTemplate.Format(context.Background(), vars.toMap())
	if err != nil {
		return "", fmt.Errorf("渲染模板失败: %w", err)
	}
	if len(messages) == 0 || messages[0] == nil {
		return "", nil
	}

	return messages[0].Content, nil
}


func (v *Variables) toMap() map[string]any {
	result := map[string]any{
		"Query":          v.Query,
		"Contexts":       v.Contexts,
		"CurrentTime":    v.CurrentTime,
		"CurrentWeek":    v.CurrentWeek,
		"KnowledgeBases": v.KnowledgeBases,
	}

	for k, val := range v.CustomVars {
		if strings.TrimSpace(k) == "" {
			continue
		}
		result[k] = val
	}

	return result
}

func (m *Manager) RenderText(content string, vars *Variables) (string, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return "", nil
	}
	return m.Render(&Template{ID: "inline", Name: "inline", Content: content}, vars)
}

// RenderSystemPrompt 渲染系统提示词
func (m *Manager) RenderSystemPrompt(id string, vars *Variables) (string, error) {
	tpl, ok := m.GetSystemPrompt(id)
	if !ok {
		tpl, _ = m.GetSystemPrompt("default")
	}
	return m.Render(tpl, vars)
}

// RenderContext 渲染上下文
func (m *Manager) RenderContext(id string, query string, contexts string) (string, error) {
	tpl, ok := m.GetContextTemplate(id)
	if !ok {
		tpl, _ = m.GetContextTemplate("default")
	}
	return m.Render(tpl, &Variables{
		Query:    query,
		Contexts: contexts,
	})
}

// RenderRewrite 渲染查询重写提示词
func (m *Manager) RenderRewrite(id string, query string) (string, error) {
	tpl, ok := m.GetRewriteTemplate(id)
	if !ok {
		tpl, _ = m.GetRewriteTemplate("default")
	}
	return m.Render(tpl, &Variables{
		Query: query,
	})
}

// ListSystemPrompts 列出所有系统提示词模板
func (m *Manager) ListSystemPrompts() []*Template {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Template, 0, len(m.systemPrompts))
	for _, t := range m.systemPrompts {
		result = append(result, t)
	}
	return result
}

// FormatContexts 格式化检索结果为上下文字符串
func FormatContexts(docs []DocumentContext) string {
	if len(docs) == 0 {
		return "（未找到相关参考资料）"
	}

	var sb strings.Builder
	for i, doc := range docs {
		sb.WriteString(fmt.Sprintf("### 资料 %d", i+1))
		if doc.Source != "" {
			sb.WriteString(fmt.Sprintf(" [来源: %s]", doc.Source))
		}
		sb.WriteString("\n")
		sb.WriteString(doc.Content)
		sb.WriteString("\n\n")
	}
	return sb.String()
}

// DocumentContext 文档上下文
type DocumentContext struct {
	Content string
	Source  string
	Score   float64
}
