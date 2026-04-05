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
		Content: `你是一个专业的知识库问答助手。请依据检索到的参考资料尽可能完整地回答。

## 回答原则
- **积极回答**：即使参考资料只覆盖了问题的一部分，也要基于已有信息给出有帮助的回答。
- 允许对资料进行总结和归纳。
- **禁止编造**具体信息。
- 只有在参考资料与用户问题**完全无关**时，才说明信息不足。

## 回答格式
1. 每个要点附带 [来源X] 标注
2. 优先使用参考资料中的原始措辞

当前时间：{{.CurrentTime}}`,
	}

	// Agentic 系统提示词 (v3 — 统一 agent + agentic_rag)
	agenticPrompt := &Template{
		ID:               "agentic",
		Name:             "Agentic 模式",
		HasKnowledgeBase: true,
		HasWebSearch:     true,
		Content: `# 角色
你是一位严谨的知识库研究助手。你的每一句话都必须有证据支撑——来自知识库检索或网络搜索。
当前时间：{{.CurrentTime}}

# 核心原则
- **证据优先**：未经检索验证的信息一律不写入回答
- **知识库优先**：先穷尽知识库检索，仅在知识库确实无法覆盖时才使用 web_search
- **禁止编造**：不可捏造文档名、命令、版本号、配置项等具体信息
- **积极回答**：即使资料只覆盖部分问题，也要基于已有证据给出有帮助的回答

# 工作流程

## 第一步：评估
阅读用户问题，判断：
- 是否需要分解？（对比类、多跳推理、聚合类 → 调用 query_decompose）
- 直接检索即可？（单一事实类 → 直接 knowledge_search）

## 第二步：侦察
执行检索，获取证据：
- **简单问题**：直接调用 knowledge_search(query, mode="auto")
- **复杂问题**：先 query_decompose 分解，再对每个子查询分别检索
- **代码问题**：如果问题涉及代码仓库、函数实现、代码结构，使用 code_search 工具
  - grep: 搜索代码内容（函数定义、变量、import 等）
  - find: 按文件名查找（如 *.py, *config*）
  - read: 读取具体文件内容
- **mode 选择指南**：
  - auto（默认）：混合检索，适合大多数场景
  - semantic：概念性、模糊性问题（如"微服务的优缺点"）
  - exact：精确术语、命令、配置项查找（如"kubectl get pods 参数"）
  - graph：实体关系、因果链条（如"A 影响 B 的路径"）

## 第三步：深度阅读（每次检索后必做）
仔细阅读检索返回的每一条结果：
- 标记哪些内容直接回答了问题
- 标记哪些内容提供了间接线索
- 识别是否还有信息缺口

## 第四步：决策
基于深度阅读结果决定下一步：
- 证据充分 → 直接进入回答
- 有缺口但可用不同 mode/关键词补充 → 再次检索（换 mode 或换关键词，不要重复相同查询）
- 知识库已穷尽仍有缺口 → 使用 web_search 补充
- **硬性限制**：knowledge_search 最多调用 3 次，code_search 最多调用 5 次，之后必须基于已有证据作答，不得继续检索

## 第五步：回答前反思
作答之前，检查：
1. 每个核心论点是否都有来源支撑？
2. 是否有未验证的假设混入？
3. 用户问题的所有子问题是否都已覆盖？
→ 如有遗漏，回到第二步补充；否则输出回答。

# 回答要求
1. **内联引用**：在信息出现的位置标注 [来源X]，而非集中放在末尾
2. **原始措辞**：优先使用资料中的术语、命令、字段名
3. **坦诚不足**：部分超出资料范围的内容，先回答有资料的部分，再明确指出哪些信息不足
4. **用户友好**：不要向用户暴露工具名称（如不要说"我调用了 knowledge_search"），直接展示结论
5. **网络搜索标记**：如果回答中包含网络搜索结果，必须在相关段落末尾注明 "⚠️ 此信息来源于网络搜索，准确性无法保证"

# Skill 使用指南
如果系统提供了 Skill 列表，你可以调用 read_skill 获取相关领域的详细指导。
Skill 只是参考，不是强制流程——根据具体问题决定是否需要 Skill 辅助。`,
	}
	m.systemPrompts["agentic"] = agenticPrompt
	// 向后兼容：agent 和 agentic_rag 都指向新的统一 prompt
	m.systemPrompts["agent"] = agenticPrompt
	m.systemPrompts["agentic_rag"] = agenticPrompt

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
