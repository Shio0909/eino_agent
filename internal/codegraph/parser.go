package codegraph

import (
	"context"
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/python"
	tsTypescript "github.com/smacker/go-tree-sitter/typescript/typescript"
)

// Parser 代码 AST 解析器（基于 tree-sitter）
type Parser struct{}

// NewParser 创建解析器
func NewParser() *Parser {
	return &Parser{}
}

// Parse 解析单个文件，提取代码实体和关系
func (p *Parser) Parse(ctx context.Context, filePath string, source []byte, lang Language) (*ParseResult, error) {
	sitterLang := getSitterLanguage(lang)
	if sitterLang == nil {
		return nil, fmt.Errorf("unsupported language: %s", lang)
	}

	parser := sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(sitterLang)

	tree, err := parser.ParseCtx(ctx, nil, source)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", filePath, err)
	}
	defer tree.Close()

	result := &ParseResult{
		FilePath: filePath,
		Language: string(lang),
	}

	root := tree.RootNode()

	switch lang {
	case LangPython:
		p.extractPython(root, source, filePath, result)
	case LangTypeScript, LangJavaScript:
		p.extractTypeScript(root, source, filePath, result)
	case LangGo:
		p.extractGo(root, source, filePath, result)
	}

	return result, nil
}

// getSitterLanguage 获取 tree-sitter 语言对象
func getSitterLanguage(lang Language) *sitter.Language {
	switch lang {
	case LangPython:
		return python.GetLanguage()
	case LangTypeScript:
		return tsTypescript.GetLanguage()
	case LangJavaScript:
		return javascript.GetLanguage()
	case LangGo:
		return golang.GetLanguage()
	default:
		return nil
	}
}

// ── Python 解析 ──

func (p *Parser) extractPython(root *sitter.Node, source []byte, filePath string, result *ParseResult) {
	p.walkPython(root, source, filePath, "", result)
}

func (p *Parser) walkPython(node *sitter.Node, source []byte, filePath, className string, result *ParseResult) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "function_definition":
			name := nodeText(child.ChildByFieldName("name"), source)
			params := nodeText(child.ChildByFieldName("parameters"), source)
			entity := CodeEntity{
				Name:      name,
				FilePath:  filePath,
				LineStart: int(child.StartPoint().Row) + 1,
				LineEnd:   int(child.EndPoint().Row) + 1,
				Params:    params,
			}
			if className != "" {
				entity.Type = EntityMethod
				entity.ClassName = className
				entity.QualifiedName = fmt.Sprintf("%s.%s", className, name)
			} else {
				entity.Type = EntityFunction
				entity.QualifiedName = name
			}
			// 提取装饰器
			entity.Decorators = extractPythonDecorators(node, child, source)
			result.Entities = append(result.Entities, entity)

			// 提取函数体中的调用
			body := child.ChildByFieldName("body")
			if body != nil {
				p.extractCalls(body, source, entity.QualifiedName, result)
			}

		case "class_definition":
			name := nodeText(child.ChildByFieldName("name"), source)
			entity := CodeEntity{
				Type:          EntityClass,
				Name:          name,
				QualifiedName: name,
				FilePath:      filePath,
				LineStart:     int(child.StartPoint().Row) + 1,
				LineEnd:       int(child.EndPoint().Row) + 1,
			}
			// 提取基类（继承）
			superclasses := child.ChildByFieldName("superclasses")
			if superclasses != nil {
				bases := nodeText(superclasses, source)
				entity.Decorators = bases // 暂存基类信息
				for _, base := range splitBases(bases) {
					result.Relations = append(result.Relations, CodeRelation{
						Type:   RelInherits,
						Source: name,
						Target: strings.TrimSpace(base),
					})
				}
			}
			result.Entities = append(result.Entities, entity)

			// 递归解析类体（提取方法）
			body := child.ChildByFieldName("body")
			if body != nil {
				p.walkPython(body, source, filePath, name, result)
			}

		case "import_statement", "import_from_statement":
			importText := nodeText(child, source)
			moduleName := extractPythonImportModule(importText)
			if moduleName != "" {
				result.Relations = append(result.Relations, CodeRelation{
					Type:   RelImports,
					Source: filePath,
					Target: moduleName,
				})
			}
		}
	}
}

// ── TypeScript/JavaScript 解析 ──

func (p *Parser) extractTypeScript(root *sitter.Node, source []byte, filePath string, result *ParseResult) {
	p.walkTypeScript(root, source, filePath, "", result)
}

func (p *Parser) walkTypeScript(node *sitter.Node, source []byte, filePath, className string, result *ParseResult) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "function_declaration", "arrow_function", "method_definition":
			name := nodeText(child.ChildByFieldName("name"), source)
			if name == "" {
				continue
			}
			params := nodeText(child.ChildByFieldName("parameters"), source)
			entity := CodeEntity{
				Name:      name,
				FilePath:  filePath,
				LineStart: int(child.StartPoint().Row) + 1,
				LineEnd:   int(child.EndPoint().Row) + 1,
				Params:    params,
			}
			if className != "" {
				entity.Type = EntityMethod
				entity.ClassName = className
				entity.QualifiedName = fmt.Sprintf("%s.%s", className, name)
			} else {
				entity.Type = EntityFunction
				entity.QualifiedName = name
			}
			result.Entities = append(result.Entities, entity)

			body := child.ChildByFieldName("body")
			if body != nil {
				p.extractCalls(body, source, entity.QualifiedName, result)
			}

		case "class_declaration":
			name := nodeText(child.ChildByFieldName("name"), source)
			entity := CodeEntity{
				Type:          EntityClass,
				Name:          name,
				QualifiedName: name,
				FilePath:      filePath,
				LineStart:     int(child.StartPoint().Row) + 1,
				LineEnd:       int(child.EndPoint().Row) + 1,
			}
			result.Entities = append(result.Entities, entity)

			// 提取 extends
			heritage := findChild(child, "class_heritage")
			if heritage != nil {
				baseClass := nodeText(heritage, source)
				if strings.Contains(baseClass, "extends") {
					parts := strings.SplitN(baseClass, "extends", 2)
					if len(parts) == 2 {
						baseName := strings.TrimSpace(strings.Split(parts[1], "{")[0])
						baseName = strings.Split(baseName, " ")[0] // remove "implements ..."
						result.Relations = append(result.Relations, CodeRelation{
							Type:   RelInherits,
							Source: name,
							Target: baseName,
						})
					}
				}
			}

			body := child.ChildByFieldName("body")
			if body != nil {
				p.walkTypeScript(body, source, filePath, name, result)
			}

		case "import_statement":
			importText := nodeText(child, source)
			moduleName := extractTSImportModule(importText)
			if moduleName != "" {
				result.Relations = append(result.Relations, CodeRelation{
					Type:   RelImports,
					Source: filePath,
					Target: moduleName,
				})
			}
		}
	}
}

// ── Go 解析 ──

func (p *Parser) extractGo(root *sitter.Node, source []byte, filePath string, result *ParseResult) {
	for i := 0; i < int(root.ChildCount()); i++ {
		child := root.Child(i)
		switch child.Type() {
		case "function_declaration":
			name := nodeText(child.ChildByFieldName("name"), source)
			params := nodeText(child.ChildByFieldName("parameters"), source)
			retType := nodeText(child.ChildByFieldName("result"), source)
			entity := CodeEntity{
				Type:          EntityFunction,
				Name:          name,
				QualifiedName: name,
				FilePath:      filePath,
				LineStart:     int(child.StartPoint().Row) + 1,
				LineEnd:       int(child.EndPoint().Row) + 1,
				Params:        params,
				ReturnType:    retType,
			}
			result.Entities = append(result.Entities, entity)

			body := child.ChildByFieldName("body")
			if body != nil {
				p.extractCalls(body, source, name, result)
			}

		case "method_declaration":
			name := nodeText(child.ChildByFieldName("name"), source)
			params := nodeText(child.ChildByFieldName("parameters"), source)
			receiver := nodeText(child.ChildByFieldName("receiver"), source)
			recvType := extractGoReceiverType(receiver)
			entity := CodeEntity{
				Type:          EntityMethod,
				Name:          name,
				QualifiedName: fmt.Sprintf("%s.%s", recvType, name),
				ClassName:     recvType,
				FilePath:      filePath,
				LineStart:     int(child.StartPoint().Row) + 1,
				LineEnd:       int(child.EndPoint().Row) + 1,
				Params:        params,
			}
			result.Entities = append(result.Entities, entity)

			body := child.ChildByFieldName("body")
			if body != nil {
				p.extractCalls(body, source, entity.QualifiedName, result)
			}

		case "type_declaration":
			for j := 0; j < int(child.ChildCount()); j++ {
				spec := child.Child(j)
				if spec.Type() == "type_spec" {
					typeName := nodeText(spec.ChildByFieldName("name"), source)
					typeVal := spec.ChildByFieldName("type")
					if typeVal != nil && typeVal.Type() == "struct_type" {
						entity := CodeEntity{
							Type:          EntityClass,
							Name:          typeName,
							QualifiedName: typeName,
							FilePath:      filePath,
							LineStart:     int(spec.StartPoint().Row) + 1,
							LineEnd:       int(spec.EndPoint().Row) + 1,
						}
						result.Entities = append(result.Entities, entity)
					} else if typeVal != nil && typeVal.Type() == "interface_type" {
						entity := CodeEntity{
							Type:          EntityClass,
							Name:          typeName,
							QualifiedName: typeName,
							FilePath:      filePath,
							LineStart:     int(spec.StartPoint().Row) + 1,
							LineEnd:       int(spec.EndPoint().Row) + 1,
							Decorators:    "interface",
						}
						result.Entities = append(result.Entities, entity)
					}
				}
			}

		case "import_declaration":
			for j := 0; j < int(child.ChildCount()); j++ {
				importSpec := child.Child(j)
				if importSpec.Type() == "import_spec_list" {
					for k := 0; k < int(importSpec.ChildCount()); k++ {
						spec := importSpec.Child(k)
						if spec.Type() == "import_spec" {
							path := nodeText(spec.ChildByFieldName("path"), source)
							path = strings.Trim(path, "\"")
							if path != "" {
								result.Relations = append(result.Relations, CodeRelation{
									Type:   RelImports,
									Source: filePath,
									Target: path,
								})
							}
						}
					}
				} else if importSpec.Type() == "import_spec" {
					path := nodeText(importSpec.ChildByFieldName("path"), source)
					path = strings.Trim(path, "\"")
					if path != "" {
						result.Relations = append(result.Relations, CodeRelation{
							Type:   RelImports,
							Source: filePath,
							Target: path,
						})
					}
				}
			}
		}
	}
}

// ── 通用 helper ──

// extractCalls 递归提取调用关系
func (p *Parser) extractCalls(node *sitter.Node, source []byte, callerName string, result *ParseResult) {
	if node == nil {
		return
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "call" || child.Type() == "call_expression" {
			fn := child.ChildByFieldName("function")
			if fn != nil {
				callee := nodeText(fn, source)
				if callee != "" && callee != callerName {
					result.Relations = append(result.Relations, CodeRelation{
						Type:   RelCalls,
						Source: callerName,
						Target: callee,
					})
				}
			}
		} else {
			p.extractCalls(child, source, callerName, result)
		}
	}
}

func nodeText(node *sitter.Node, source []byte) string {
	if node == nil {
		return ""
	}
	return string(source[node.StartByte():node.EndByte()])
}

func findChild(node *sitter.Node, childType string) *sitter.Node {
	for i := 0; i < int(node.ChildCount()); i++ {
		c := node.Child(i)
		if c.Type() == childType {
			return c
		}
	}
	return nil
}

func splitBases(bases string) []string {
	bases = strings.TrimPrefix(bases, "(")
	bases = strings.TrimSuffix(bases, ")")
	parts := strings.Split(bases, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func extractPythonDecorators(parent, funcNode *sitter.Node, source []byte) string {
	// tree-sitter Python 中装饰器是 function_definition 前的 decorator 节点
	var decs []string
	for i := 0; i < int(parent.ChildCount()); i++ {
		c := parent.Child(i)
		if c == funcNode {
			break
		}
		if c.Type() == "decorator" {
			decs = append(decs, nodeText(c, source))
		}
	}
	if len(decs) == 0 {
		return ""
	}
	return strings.Join(decs, ", ")
}

func extractPythonImportModule(importText string) string {
	// "import os" → "os"
	// "from os.path import join" → "os.path"
	importText = strings.TrimSpace(importText)
	if strings.HasPrefix(importText, "from ") {
		parts := strings.Fields(importText)
		if len(parts) >= 2 {
			return parts[1]
		}
	} else if strings.HasPrefix(importText, "import ") {
		parts := strings.Fields(importText)
		if len(parts) >= 2 {
			return strings.Split(parts[1], ",")[0]
		}
	}
	return ""
}

func extractTSImportModule(importText string) string {
	// import ... from "module"
	if idx := strings.Index(importText, "from "); idx >= 0 {
		rest := importText[idx+5:]
		rest = strings.Trim(rest, " ;\"'`")
		return rest
	}
	return ""
}

func extractGoReceiverType(receiver string) string {
	// (s *Server) → Server
	receiver = strings.TrimPrefix(receiver, "(")
	receiver = strings.TrimSuffix(receiver, ")")
	parts := strings.Fields(receiver)
	if len(parts) >= 2 {
		return strings.TrimPrefix(parts[1], "*")
	}
	if len(parts) == 1 {
		return strings.TrimPrefix(parts[0], "*")
	}
	return receiver
}
