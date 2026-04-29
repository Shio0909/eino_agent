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
	Query          string                 // 用户查询
	Contexts       string                 // 检索到的上下文
	CurrentTime    string                 // 当前时间
	CurrentWeek    string                 // 当前星期
	KnowledgeBases []KnowledgeBaseInfo    // 知识库信息
	CustomVars     map[string]interface{} // 自定义变量
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
		Content: `你是一个专业的知识库问答助手。请尽可能利用参考资料回答用户问题。

## 回答原则
- **积极回答**：即使参考资料只覆盖了问题的一部分，也要基于已有信息给出有帮助的回答。
- 允许对资料内容进行总结、归纳、对比和组织。
- **禁止编造**具体信息（文档名、命令、版本号、配置项等）。
- 只有在参考资料与用户问题**完全无关**时，才说明信息不足。

## 回答格式
1. 每个要点附带 [来源X] 标注，指向对应参考资料编号
2. 优先使用参考资料中的原始措辞和术语

当前时间：{{.CurrentTime}}`,
	}

	// RAG 系统提示词
	m.systemPrompts["rag"] = &Template{
		ID:               "rag",
		Name:             "RAG 问答模式",
		HasKnowledgeBase: true,
		Content: `你是一个专业的知识库问答助手。请依据检索到的参考资料尽可能完整地回答。

## 回答原则
- **积极回答**：即使参考资料只覆盖了问题的一部分，也要基于已有信息给出有帮助的回答。
- 允许对资料内容进行总结、归纳和组织。
- **禁止编造**具体信息（文档名、命令、版本号、配置项等）。
- 只有在参考资料与用户问题**完全无关**时，才说明信息不足。

## 回答格式
1. 每个要点附带 [来源X] 标注
2. 优先使用参考资料中的原始措辞

当前时间：{{.CurrentTime}}`,
	}

	// Pipeline 系统提示词
	m.systemPrompts["pipeline"] = &Template{
		ID:               "pipeline",
		Name:             "Pipeline 模式",
		HasKnowledgeBase: true,
		Content: `你是一个专业的知识库问答助手。你只能依据当前请求已检索到的知识库参考资料回答。

## 回答原则
- **仅用上下文**：不得使用训练知识、实时网络信息或未出现在参考资料中的事实补充答案。
- **积极回答**：如果参考资料只覆盖部分问题，先回答有依据的部分，再说明缺失信息。
- **禁止编造**：不得捏造文档名、命令、版本号、配置项、数字或结论。
- **坦诚不足**：当参考资料为空或与问题无关时，明确说明当前知识库没有足够依据。

## 回答格式
1. 每个关键结论附带 [来源X] 标注
2. 优先使用参考资料中的原始术语和措辞
3. 不要提到网络搜索或工具调用

当前时间：{{.CurrentTime}}`,
	}

	// Agentic 系统提示词
	agenticPrompt := &Template{
		ID:               "agentic",
		Name:             "Agentic 模式",
		HasKnowledgeBase: true,
		HasWebSearch:     true,
		Content: `# 角色
	你是一位严谨的 ReAct 知识库助手。你通过可用工具获取证据，再基于证据回答。
	当前时间：{{.CurrentTime}}

	# 不可违反的规则
	- 回答事实性、知识库、文档、代码或项目问题前，必须先使用合适的检索工具获取证据。
	- 未经工具或用户输入验证的信息，不得写入最终回答。
	- 不得捏造文档名、命令、版本号、配置项、数字、来源或结论。
	- 资料不足时，先回答已有证据支持的部分，再明确说明缺失信息。
	- 不要向用户暴露工具调用过程或内部工作流名称。

	# 工具底线
	- 知识库问题优先使用 knowledge_search。
	- 普通知识库检索结果为空、过少或明显不相关时，才使用 knowledge_search_hyde；HyDE 生成内容不能作为最终答案依据。
	- 多实体、对比、多跳或聚合问题可先使用 query_decompose。
	- 代码实现、文件、函数、仓库结构问题使用代码检索工具。
	- 只有用户需要实时、外部或公开网络信息，且 web_search 可用时，才使用 web_search，并在答案中标明网络来源不保证准确。
	- 如果系统提供 Skill，可按需调用 read_skill 获取具体工作流说明；Skill 是流程参考，不替代以上硬规则。

	# 回答要求
	1. 在信息出现的位置标注 [来源X]，不要只在末尾集中列来源。
	2. 优先使用资料中的原始术语、命令、字段名和数值。
	3. 若来源冲突，明确列出冲突点和各自来源，不要只采信一方。
	4. 若没有足够证据，直接说明当前资料不足，不要输出普通科普答案。`,
	}
	m.systemPrompts["agentic"] = agenticPrompt

	// 默认上下文模板
	m.contextTpls["default"] = &Template{
		ID:   "default",
		Name: "默认上下文模板",
		Content: `## 参考资料
{{.Contexts}}

## 用户问题
{{.Query}}

请基于上述参考资料尽可能完整地回答。每个要点附带 [来源X] 出处。优先使用资料原文措辞。允许对资料进行总结归纳。只有在资料与问题完全无关时才说明信息不足。`,
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
