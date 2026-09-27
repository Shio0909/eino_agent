package codegraph

import (
	"context"
	"fmt"
	"path"
	"regexp"
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

// fileScope 单文件的解析上下文：包名与导入别名表，用于把调用文本规范化为 CallSite
type fileScope struct {
	filePath string
	lang     Language
	pkg      string
	// imports 本地别名 → 导入路径/模块（Go 包别名、Python `import x as y`、TS 命名空间/默认导入）
	imports map[string]string
	// symbols 直接导入的符号：本地名 → {模块, 原名}（Python `from x import y`、TS `import {y} from "x"`）
	symbols map[string]importedSymbol
}

type importedSymbol struct {
	module string
	name   string
}

// callScope 函数体内的调用上下文
type callScope struct {
	*fileScope
	caller    string // 调用方 qualified_name
	className string // 所属类 / Go 接收者类型
	recvVar   string // Go 接收者变量名（如 s *Server 中的 s）
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

	scope := &fileScope{
		filePath: filePath,
		lang:     lang,
		pkg:      packageKey(filePath, lang),
		imports:  map[string]string{},
		symbols:  map[string]importedSymbol{},
	}
	result := &ParseResult{
		FilePath: filePath,
		Language: string(lang),
		Package:  scope.pkg,
	}

	root := tree.RootNode()

	switch lang {
	case LangPython:
		p.collectPythonImports(root, source, scope)
		p.walkPython(root, source, scope, "", result)
	case LangTypeScript, LangJavaScript:
		p.collectTSImports(root, source, scope)
		p.walkTypeScript(root, source, scope, "", result)
	case LangGo:
		p.extractGo(root, source, scope, result)
	}

	for i := range result.Entities {
		result.Entities[i].Package = scope.pkg
	}
	appendStructuralRelations(result)
	return result, nil
}

// packageKey 返回实体所属包的键：Go 按目录，其余语言按模块文件（去扩展名）
func packageKey(filePath string, lang Language) string {
	filePath = path.Clean(strings.ReplaceAll(filePath, "\\", "/"))
	if lang == LangGo {
		return path.Dir(filePath)
	}
	return strings.TrimSuffix(filePath, path.Ext(filePath))
}

// appendStructuralRelations 为实体补充 File -DEFINES-> Entity 与 Class -CONTAINS-> Method 关系
func appendStructuralRelations(result *ParseResult) {
	for _, e := range result.Entities {
		result.Relations = append(result.Relations, CodeRelation{
			Type:       RelDefines,
			Source:     result.FilePath,
			Target:     e.QualifiedName,
			SourceFile: result.FilePath,
			TargetFile: e.FilePath,
		})
		if e.Type == EntityMethod && e.ClassName != "" {
			result.Relations = append(result.Relations, CodeRelation{
				Type:       RelContains,
				Source:     e.ClassName,
				Target:     e.QualifiedName,
				TargetFile: e.FilePath,
			})
		}
	}
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

func (p *Parser) collectPythonImports(root *sitter.Node, source []byte, scope *fileScope) {
	for i := 0; i < int(root.ChildCount()); i++ {
		child := root.Child(i)
		switch child.Type() {
		case "import_statement":
			for j := 0; j < int(child.NamedChildCount()); j++ {
				item := child.NamedChild(j)
				switch item.Type() {
				case "dotted_name":
					module := nodeText(item, source)
					scope.imports[module] = module
					// `import a.b` 在本地绑定的是顶层名 a
					if top := strings.SplitN(module, ".", 2)[0]; top != module {
						if _, exists := scope.imports[top]; !exists {
							scope.imports[top] = top
						}
					}
				case "aliased_import":
					module := nodeText(item.ChildByFieldName("name"), source)
					alias := nodeText(item.ChildByFieldName("alias"), source)
					if module != "" && alias != "" {
						scope.imports[alias] = module
					}
				}
			}
		case "import_from_statement":
			module := nodeText(child.ChildByFieldName("module_name"), source)
			if module == "" {
				continue
			}
			moduleNode := child.ChildByFieldName("module_name")
			for j := 0; j < int(child.NamedChildCount()); j++ {
				item := child.NamedChild(j)
				if item.StartByte() == moduleNode.StartByte() {
					continue
				}
				switch item.Type() {
				case "dotted_name":
					name := nodeText(item, source)
					scope.symbols[name] = importedSymbol{module: module, name: name}
				case "aliased_import":
					name := nodeText(item.ChildByFieldName("name"), source)
					alias := nodeText(item.ChildByFieldName("alias"), source)
					if name != "" && alias != "" {
						scope.symbols[alias] = importedSymbol{module: module, name: name}
					}
				}
			}
		}
	}
}

func (p *Parser) walkPython(node *sitter.Node, source []byte, scope *fileScope, className string, result *ParseResult) {
	filePath := scope.filePath
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "decorated_definition":
			p.walkPython(child, source, scope, className, result)

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
				p.extractCalls(body, source, &callScope{fileScope: scope, caller: entity.QualifiedName, className: className}, result)
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
						Type:       RelInherits,
						Source:     name,
						Target:     strings.TrimSpace(base),
						SourceFile: filePath,
					})
				}
			}
			result.Entities = append(result.Entities, entity)

			// 递归解析类体（提取方法）
			body := child.ChildByFieldName("body")
			if body != nil {
				p.walkPython(body, source, scope, name, result)
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

func (p *Parser) collectTSImports(root *sitter.Node, source []byte, scope *fileScope) {
	var visit func(n *sitter.Node)
	visit = func(n *sitter.Node) {
		for i := 0; i < int(n.ChildCount()); i++ {
			child := n.Child(i)
			switch child.Type() {
			case "import_statement":
				p.collectTSImportStatement(child, source, scope)
			case "export_statement":
				visit(child)
			}
		}
	}
	visit(root)
}

func (p *Parser) collectTSImportStatement(stmt *sitter.Node, source []byte, scope *fileScope) {
	module := strings.Trim(nodeText(stmt.ChildByFieldName("source"), source), "\"'`")
	if module == "" {
		return
	}
	clause := findChild(stmt, "import_clause")
	if clause == nil {
		return
	}
	for i := 0; i < int(clause.NamedChildCount()); i++ {
		item := clause.NamedChild(i)
		switch item.Type() {
		case "identifier": // import Foo from "x"
			scope.imports[nodeText(item, source)] = module
		case "namespace_import": // import * as ns from "x"
			if id := findChild(item, "identifier"); id != nil {
				scope.imports[nodeText(id, source)] = module
			}
		case "named_imports": // import { a, b as c } from "x"
			for j := 0; j < int(item.NamedChildCount()); j++ {
				spec := item.NamedChild(j)
				if spec.Type() != "import_specifier" {
					continue
				}
				name := nodeText(spec.ChildByFieldName("name"), source)
				local := nodeText(spec.ChildByFieldName("alias"), source)
				if local == "" {
					local = name
				}
				if name != "" {
					scope.symbols[local] = importedSymbol{module: module, name: name}
				}
			}
		}
	}
}

func (p *Parser) walkTypeScript(node *sitter.Node, source []byte, scope *fileScope, className string, result *ParseResult) {
	filePath := scope.filePath
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
				p.extractCalls(body, source, &callScope{fileScope: scope, caller: entity.QualifiedName, className: className}, result)
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
							Type:       RelInherits,
							Source:     name,
							Target:     baseName,
							SourceFile: filePath,
						})
					}
				}
			}

			body := child.ChildByFieldName("body")
			if body != nil {
				p.walkTypeScript(body, source, scope, name, result)
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

		case "lexical_declaration", "variable_declaration":
			p.extractVarDeclFunctions(child, source, scope, className, result)

		case "export_statement":
			// Walk into the exported declaration (e.g. export function foo, export class Bar)
			p.walkTypeScript(child, source, scope, className, result)

		case "interface_declaration":
			name := nodeText(child.ChildByFieldName("name"), source)
			if name == "" {
				continue
			}
			result.Entities = append(result.Entities, CodeEntity{
				Type:          EntityInterface,
				Name:          name,
				QualifiedName: name,
				FilePath:      filePath,
				LineStart:     int(child.StartPoint().Row) + 1,
				LineEnd:       int(child.EndPoint().Row) + 1,
			})

		case "type_alias_declaration":
			name := nodeText(child.ChildByFieldName("name"), source)
			if name == "" {
				continue
			}
			result.Entities = append(result.Entities, CodeEntity{
				Type:          EntityClass,
				Name:          name,
				QualifiedName: name,
				FilePath:      filePath,
				LineStart:     int(child.StartPoint().Row) + 1,
				LineEnd:       int(child.EndPoint().Row) + 1,
			})
		}
	}
}

// extractVarDeclFunctions handles patterns like:
//
//	const fetchData = () => { ... }
//	const fn = function() { ... }
func (p *Parser) extractVarDeclFunctions(node *sitter.Node, source []byte, scope *fileScope, className string, result *ParseResult) {
	for i := 0; i < int(node.ChildCount()); i++ {
		decl := node.Child(i)
		if decl.Type() != "variable_declarator" {
			continue
		}
		nameNode := decl.ChildByFieldName("name")
		valueNode := decl.ChildByFieldName("value")
		if nameNode == nil || valueNode == nil {
			continue
		}
		valType := valueNode.Type()
		if valType != "arrow_function" && valType != "function" && valType != "function_expression" {
			continue
		}
		name := nodeText(nameNode, source)
		if name == "" {
			continue
		}
		params := nodeText(valueNode.ChildByFieldName("parameters"), source)
		entity := CodeEntity{
			Name:      name,
			FilePath:  scope.filePath,
			LineStart: int(node.StartPoint().Row) + 1,
			LineEnd:   int(node.EndPoint().Row) + 1,
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

		body := valueNode.ChildByFieldName("body")
		if body != nil {
			p.extractCalls(body, source, &callScope{fileScope: scope, caller: entity.QualifiedName, className: className}, result)
		}
	}
}

// ── Go 解析 ──

func (p *Parser) extractGo(root *sitter.Node, source []byte, scope *fileScope, result *ParseResult) {
	filePath := scope.filePath
	// 先收集导入：函数体里的 pkg.Func() 需要依赖别名表解析
	for i := 0; i < int(root.ChildCount()); i++ {
		child := root.Child(i)
		if child.Type() != "import_declaration" {
			continue
		}
		for _, spec := range goImportSpecs(child) {
			importPath := strings.Trim(nodeText(spec.ChildByFieldName("path"), source), "\"`")
			if importPath == "" {
				continue
			}
			result.Relations = append(result.Relations, CodeRelation{
				Type:   RelImports,
				Source: filePath,
				Target: importPath,
			})
			alias := nodeText(spec.ChildByFieldName("name"), source)
			switch alias {
			case "_", ".":
				// 空白导入无可调用名称；点导入的符号按同包调用做名称匹配
				continue
			case "":
				alias = defaultGoImportName(importPath)
			}
			scope.imports[alias] = importPath
		}
	}

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
				p.extractCalls(body, source, &callScope{fileScope: scope, caller: name}, result)
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
				p.extractCalls(body, source, &callScope{
					fileScope: scope,
					caller:    entity.QualifiedName,
					className: recvType,
					recvVar:   extractGoReceiverVar(receiver),
				}, result)
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
		}
	}
}

func goImportSpecs(decl *sitter.Node) []*sitter.Node {
	var specs []*sitter.Node
	for j := 0; j < int(decl.ChildCount()); j++ {
		child := decl.Child(j)
		switch child.Type() {
		case "import_spec":
			specs = append(specs, child)
		case "import_spec_list":
			for k := 0; k < int(child.ChildCount()); k++ {
				if spec := child.Child(k); spec.Type() == "import_spec" {
					specs = append(specs, spec)
				}
			}
		}
	}
	return specs
}

var goMajorVersionRE = regexp.MustCompile(`^v[0-9]+$`)

// defaultGoImportName 推断未显式命名的 Go 导入在源码中的包名（尽力而为：取路径末段，
// 跳过 /vN 主版本后缀，去掉 gopkg.in 风格的 .vN 后缀与 go- 前缀）
func defaultGoImportName(importPath string) string {
	parts := strings.Split(importPath, "/")
	name := parts[len(parts)-1]
	if goMajorVersionRE.MatchString(name) && len(parts) > 1 {
		name = parts[len(parts)-2]
	}
	if idx := strings.Index(name, ".v"); idx > 0 && goMajorVersionRE.MatchString(name[idx+1:]) {
		name = name[:idx]
	}
	name = strings.TrimPrefix(name, "go-")
	return strings.ReplaceAll(name, "-", "")
}

// ── 调用提取 ──

// extractCalls 递归提取调用点；参数与链式调用中的嵌套调用同样会被提取
func (p *Parser) extractCalls(node *sitter.Node, source []byte, cs *callScope, result *ParseResult) {
	if node == nil {
		return
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		var fn *sitter.Node
		switch child.Type() {
		case "call", "call_expression":
			fn = child.ChildByFieldName("function")
		case "new_expression": // TS/JS: new Foo()
			fn = child.ChildByFieldName("constructor")
		}
		if fn != nil {
			if site, ok := p.analyzeCallee(fn, source, cs); ok {
				result.Calls = append(result.Calls, site)
				result.Relations = append(result.Relations, CodeRelation{
					Type:       RelCalls,
					Source:     cs.caller,
					Target:     site.Target(),
					SourceFile: cs.filePath,
				})
			}
		}
		p.extractCalls(child, source, cs, result)
	}
}

// analyzeCallee 把调用表达式的 function 部分规范化为 CallSite：
// 去掉 receiver / 包选择器，只保留被调用名，并记录导入路径或已知接收者类型
func (p *Parser) analyzeCallee(fn *sitter.Node, source []byte, cs *callScope) (CallSite, bool) {
	site := CallSite{Caller: cs.caller, CallerFile: cs.filePath}
	if fn == nil {
		return site, false
	}
	switch fn.Type() {
	case "identifier":
		site.Name = nodeText(fn, source)
		if sym, ok := cs.symbols[site.Name]; ok {
			site.Name = sym.name
			site.Qualifier = sym.module
		}

	case "selector_expression", "attribute", "member_expression":
		var operand, field *sitter.Node
		switch fn.Type() {
		case "selector_expression": // Go: operand.field
			operand, field = fn.ChildByFieldName("operand"), fn.ChildByFieldName("field")
		case "attribute": // Python: object.attribute
			operand, field = fn.ChildByFieldName("object"), fn.ChildByFieldName("attribute")
		default: // TS/JS: object.property
			operand, field = fn.ChildByFieldName("object"), fn.ChildByFieldName("property")
		}
		site.Name = nodeText(field, source)
		operandText := nodeText(operand, source)
		switch {
		case operand == nil:
			site.IsMethod = true
		case isSelfReference(operand.Type(), operandText, cs):
			site.Receiver = cs.className
			site.IsMethod = true
		default:
			if module, ok := cs.imports[operandText]; ok {
				site.Qualifier = module
			} else {
				site.IsMethod = true
			}
		}

	case "index_expression", "generic_type", "parenthesized_expression":
		// 泛型实例化 Foo[T](...) 或 (x.Foo)(...)：取其中最后一个标识符
		site.Name = lastIdentifier(nodeText(fn, source))

	default:
		return site, false
	}

	if site.Name == "" || site.Name == "super" {
		return site, false
	}
	if site.Name == cs.caller && site.Qualifier == "" && site.Receiver == "" && !site.IsMethod {
		return site, false
	}
	return site, true
}

func isSelfReference(nodeType, text string, cs *callScope) bool {
	if cs.className == "" {
		return false
	}
	switch cs.lang {
	case LangGo:
		return cs.recvVar != "" && nodeType == "identifier" && text == cs.recvVar
	case LangPython:
		return text == "self" || text == "cls"
	default:
		return nodeType == "this"
	}
}

var identRE = regexp.MustCompile(`[A-Za-z_$][A-Za-z0-9_$]*`)

func lastIdentifier(text string) string {
	if idx := strings.Index(text, "["); idx >= 0 {
		text = text[:idx]
	}
	ids := identRE.FindAllString(text, -1)
	if len(ids) == 0 {
		return ""
	}
	return ids[len(ids)-1]
}

// ── 通用 helper ──

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
	// (s *Server) → Server；(s *Cache[K, V]) → Cache
	receiver = strings.TrimPrefix(receiver, "(")
	receiver = strings.TrimSuffix(receiver, ")")
	parts := strings.Fields(receiver)
	typ := receiver
	if len(parts) >= 2 {
		typ = parts[1]
	} else if len(parts) == 1 {
		typ = parts[0]
	}
	typ = strings.TrimPrefix(typ, "*")
	if idx := strings.Index(typ, "["); idx >= 0 {
		typ = typ[:idx]
	}
	return typ
}

func extractGoReceiverVar(receiver string) string {
	// (s *Server) → s；(*Server) → ""
	receiver = strings.TrimPrefix(receiver, "(")
	receiver = strings.TrimSuffix(receiver, ")")
	parts := strings.Fields(receiver)
	if len(parts) >= 2 {
		return parts[0]
	}
	return ""
}
