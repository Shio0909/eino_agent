// Package codegraph 代码知识图谱 — AST 解析 + Neo4j 存储
//
// 使用 tree-sitter 解析多语言代码，提取函数/类/调用/导入等结构，
// 写入 Neo4j 图数据库支持调用链分析、依赖追踪等代码智能查询。
package codegraph

// EntityType 代码实体类型
type EntityType string

const (
	EntityFunction  EntityType = "Function"
	EntityClass     EntityType = "Class"
	EntityMethod    EntityType = "Method"
	EntityModule    EntityType = "Module" // 被导入的外部/内部模块节点（IMPORTS 的目标）
	EntityFile      EntityType = "File"   // 源文件节点（DEFINES 的起点）
	EntityInterface EntityType = "Interface"
)

// RelationType 代码关系类型
type RelationType string

const (
	RelCalls    RelationType = "CALLS"    // 函数/方法 → 被调用的函数/方法
	RelImports  RelationType = "IMPORTS"  // File → Module
	RelContains RelationType = "CONTAINS" // Class → Method
	RelInherits RelationType = "INHERITS" // Class → 基类
	RelDefines  RelationType = "DEFINES"  // File → 文件中定义的实体
)

// CodeEntity 代码实体（函数、类、方法等）
type CodeEntity struct {
	Type          EntityType `json:"type"`
	Name          string     `json:"name"`
	QualifiedName string     `json:"qualified_name"` // 包内限定名: Class.method / Type.Method / func
	FilePath      string     `json:"file_path"`
	LineStart     int        `json:"line_start"`
	LineEnd       int        `json:"line_end"`
	Params        string     `json:"params,omitempty"` // 函数参数签名
	ReturnType    string     `json:"return_type,omitempty"`
	Decorators    string     `json:"decorators,omitempty"` // Python 装饰器 / Go 注解
	ClassName     string     `json:"class_name,omitempty"` // Method 所属类名
	// Package 实体所属包/模块：Go 为文件所在目录，Python/TS/JS 为去掉扩展名的文件路径。
	// QualifiedName 只在包内唯一，跨包需结合 Package/FilePath 区分。
	Package string `json:"package,omitempty"`
}

// CodeRelation 代码关系（调用、导入、继承等）
type CodeRelation struct {
	Type   RelationType `json:"type"`
	Source string       `json:"source"` // 源实体 qualified_name / 文件路径
	Target string       `json:"target"` // 目标实体 qualified_name / 模块名
	// SourceFile/TargetFile 用于在同名实体间精确定位端点；为空时仅按 qualified_name 匹配。
	SourceFile string `json:"source_file,omitempty"`
	TargetFile string `json:"target_file,omitempty"`
}

// CallSite 解析阶段得到的一次调用（尚未解析到仓库内的具体实体）
type CallSite struct {
	Caller     string `json:"caller"`      // 调用方实体 qualified_name
	CallerFile string `json:"caller_file"` // 调用方所在文件
	Name       string `json:"name"`        // 被调用的裸名称（已去掉 receiver/包选择器）
	// Qualifier 通过导入别名调用时的导入路径（Go import path / Python 模块 / TS 模块说明符）
	Qualifier string `json:"qualifier,omitempty"`
	// Receiver 已知的接收者类型（Go 方法接收者变量、Python self/cls、TS this）
	Receiver string `json:"receiver,omitempty"`
	// IsMethod 通过非包值的选择器调用（x.Foo()），目标应为方法
	IsMethod bool `json:"is_method,omitempty"`
}

// Target 返回调用目标的规范化文本形式（未解析时写入 ParseResult.Relations 的 Target）
func (c CallSite) Target() string {
	switch {
	case c.Qualifier != "":
		return c.Qualifier + "." + c.Name
	case c.Receiver != "":
		return c.Receiver + "." + c.Name
	default:
		return c.Name
	}
}

// ParseResult 单文件解析结果
type ParseResult struct {
	FilePath  string         `json:"file_path"`
	Language  string         `json:"language"`
	Package   string         `json:"package,omitempty"` // 同 CodeEntity.Package
	Entities  []CodeEntity   `json:"entities"`
	Relations []CodeRelation `json:"relations"`
	// Calls 结构化的调用点，供 CallResolver 解析为仓库内实体之间的 CALLS 关系
	Calls []CallSite `json:"calls,omitempty"`
}

// Language 支持的编程语言
type Language string

const (
	LangPython     Language = "python"
	LangTypeScript Language = "typescript"
	LangJavaScript Language = "javascript"
	LangGo         Language = "go"
)

// DetectLanguage 根据文件扩展名检测语言
func DetectLanguage(ext string) (Language, bool) {
	switch ext {
	case ".py":
		return LangPython, true
	case ".ts", ".tsx":
		return LangTypeScript, true
	case ".js", ".jsx":
		return LangJavaScript, true
	case ".go":
		return LangGo, true
	default:
		return "", false
	}
}
