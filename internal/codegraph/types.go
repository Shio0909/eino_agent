// Package codegraph 代码知识图谱 — AST 解析 + Neo4j 存储
//
// 使用 tree-sitter 解析多语言代码，提取函数/类/调用/导入等结构，
// 写入 Neo4j 图数据库支持调用链分析、依赖追踪等代码智能查询。
package codegraph

// EntityType 代码实体类型
type EntityType string

const (
	EntityFunction EntityType = "Function"
	EntityClass    EntityType = "Class"
	EntityMethod   EntityType = "Method"
	EntityModule   EntityType = "Module"
	EntityFile     EntityType = "File"
)

// RelationType 代码关系类型
type RelationType string

const (
	RelCalls    RelationType = "CALLS"
	RelImports  RelationType = "IMPORTS"
	RelContains RelationType = "CONTAINS"
	RelInherits RelationType = "INHERITS"
	RelDefines  RelationType = "DEFINES"
)

// CodeEntity 代码实体（函数、类、方法等）
type CodeEntity struct {
	Type          EntityType `json:"type"`
	Name          string     `json:"name"`
	QualifiedName string     `json:"qualified_name"` // 完全限定名: module.Class.method
	FilePath      string     `json:"file_path"`
	LineStart     int        `json:"line_start"`
	LineEnd       int        `json:"line_end"`
	Params        string     `json:"params,omitempty"`    // 函数参数签名
	ReturnType    string     `json:"return_type,omitempty"`
	Decorators    string     `json:"decorators,omitempty"` // Python 装饰器 / Go 注解
	ClassName     string     `json:"class_name,omitempty"` // Method 所属类名
}

// CodeRelation 代码关系（调用、导入、继承等）
type CodeRelation struct {
	Type   RelationType `json:"type"`
	Source string       `json:"source"` // 源实体 qualified_name
	Target string       `json:"target"` // 目标实体 qualified_name / 模块名
}

// ParseResult 单文件解析结果
type ParseResult struct {
	FilePath  string         `json:"file_path"`
	Language  string         `json:"language"`
	Entities  []CodeEntity   `json:"entities"`
	Relations []CodeRelation `json:"relations"`
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
