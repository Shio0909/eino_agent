package codegraph

import (
	"bufio"
	"bytes"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// CallResolver 把解析阶段的 CallSite 尽力解析到仓库内定义的具体实体，生成 CALLS 关系。
//
// 解析顺序：
//  1. 通过导入别名调用（pkg.Bar / from x import y）→ 定位导入的包/模块 → 在其中按名称匹配
//  2. 已知接收者类型（Go 接收者变量、self、this）→ 同包内 Type.Method
//  3. 同包/同模块按名称匹配（选择器调用只匹配方法，普通调用匹配函数/类）
//  4. 仓库内按名称唯一匹配（Go 的普通调用不做这一步：未限定的调用只能指向同包）
//
// 候选有歧义时不建边，宁缺毋滥。
type CallResolver struct {
	modulePath string
	indexes    map[string]*symbolIndex
}

type symbolIndex struct {
	byPkg   map[string]map[string][]*CodeEntity // package → name → entities
	byQName map[string]map[string]*CodeEntity   // package → qualified_name → entity
	byName  map[string][]*CodeEntity
	pkgs    []string
}

// NewCallResolver 基于整个仓库的解析结果构建符号表。
// modulePath 为 Go module 路径（go.mod 中的 module，可为空，此时按导入路径后缀匹配目录）。
func NewCallResolver(modulePath string, results []*ParseResult) *CallResolver {
	r := &CallResolver{modulePath: strings.TrimSpace(modulePath), indexes: map[string]*symbolIndex{}}
	for _, res := range results {
		if res == nil {
			continue
		}
		idx := r.index(Language(res.Language))
		pkg := res.Package
		if pkg == "" {
			pkg = packageKey(res.FilePath, Language(res.Language))
		}
		for i := range res.Entities {
			e := &res.Entities[i]
			if e.Name == "" {
				continue
			}
			if idx.byPkg[pkg] == nil {
				idx.byPkg[pkg] = map[string][]*CodeEntity{}
				idx.byQName[pkg] = map[string]*CodeEntity{}
			}
			idx.byPkg[pkg][e.Name] = append(idx.byPkg[pkg][e.Name], e)
			if _, exists := idx.byQName[pkg][e.QualifiedName]; !exists {
				idx.byQName[pkg][e.QualifiedName] = e
			}
			idx.byName[e.Name] = append(idx.byName[e.Name], e)
		}
	}
	for _, idx := range r.indexes {
		for pkg := range idx.byPkg {
			idx.pkgs = append(idx.pkgs, pkg)
		}
		sort.Strings(idx.pkgs)
	}
	return r
}

func langFamily(lang Language) string {
	if lang == LangJavaScript {
		return string(LangTypeScript)
	}
	return string(lang)
}

func (r *CallResolver) index(lang Language) *symbolIndex {
	key := langFamily(lang)
	idx, ok := r.indexes[key]
	if !ok {
		idx = &symbolIndex{
			byPkg:   map[string]map[string][]*CodeEntity{},
			byQName: map[string]map[string]*CodeEntity{},
			byName:  map[string][]*CodeEntity{},
		}
		r.indexes[key] = idx
	}
	return idx
}

// Resolve 解析单个调用点，返回被调用的实体
func (r *CallResolver) Resolve(lang Language, site CallSite) (*CodeEntity, bool) {
	idx, ok := r.indexes[langFamily(lang)]
	if !ok || site.Name == "" {
		return nil, false
	}
	callerPkg := packageKey(site.CallerFile, lang)

	if site.Qualifier != "" {
		pkg, ok := r.resolveImport(idx, lang, site.Qualifier, site.CallerFile)
		if !ok {
			return nil, false
		}
		return unique(filterKinds(idx.byPkg[pkg][site.Name], lang, false))
	}

	if site.Receiver != "" {
		if e, ok := idx.byQName[callerPkg][site.Receiver+"."+site.Name]; ok {
			return e, true
		}
	}

	method := site.IsMethod || site.Receiver != ""
	if e, ok := unique(filterKinds(idx.byPkg[callerPkg][site.Name], lang, method)); ok {
		return e, true
	}
	if lang == LangGo && !method {
		return nil, false
	}
	return unique(filterKinds(idx.byName[site.Name], lang, method))
}

// ResolveAll 解析所有调用点，返回去重后的 CALLS 关系（带两端文件路径，保证端点精确）
func (r *CallResolver) ResolveAll(results []*ParseResult) []CodeRelation {
	seen := map[string]bool{}
	var relations []CodeRelation
	for _, res := range results {
		if res == nil {
			continue
		}
		lang := Language(res.Language)
		for _, site := range res.Calls {
			target, ok := r.Resolve(lang, site)
			if !ok {
				continue
			}
			if target.QualifiedName == site.Caller && target.FilePath == site.CallerFile {
				continue
			}
			rel := CodeRelation{
				Type:       RelCalls,
				Source:     site.Caller,
				Target:     target.QualifiedName,
				SourceFile: site.CallerFile,
				TargetFile: target.FilePath,
			}
			key := rel.SourceFile + "\x00" + rel.Source + "\x00" + rel.TargetFile + "\x00" + rel.Target
			if seen[key] {
				continue
			}
			seen[key] = true
			relations = append(relations, rel)
		}
	}
	return relations
}

// filterKinds 选择器调用只匹配方法；普通/限定调用匹配函数，非 Go 语言还允许匹配类（构造调用）
func filterKinds(cands []*CodeEntity, lang Language, method bool) []*CodeEntity {
	out := make([]*CodeEntity, 0, len(cands))
	for _, e := range cands {
		switch {
		case method && e.Type == EntityMethod:
			out = append(out, e)
		case !method && e.Type == EntityFunction:
			out = append(out, e)
		case !method && lang != LangGo && e.Type == EntityClass:
			out = append(out, e)
		}
	}
	return out
}

func unique(cands []*CodeEntity) (*CodeEntity, bool) {
	if len(cands) != 1 {
		return nil, false
	}
	return cands[0], true
}

// resolveImport 把导入路径/模块说明符映射到仓库内的包键
func (r *CallResolver) resolveImport(idx *symbolIndex, lang Language, qualifier, callerFile string) (string, bool) {
	switch lang {
	case LangGo:
		if r.modulePath != "" {
			if qualifier == r.modulePath {
				return idx.hasPkg(".")
			}
			if rest, ok := strings.CutPrefix(qualifier, r.modulePath+"/"); ok {
				return idx.hasPkg(rest)
			}
			// 已知 module 路径时，其它导入都是外部依赖
			return "", false
		}
		return idx.longestSuffixPkg(qualifier, false)

	case LangPython:
		callerDir := path.Dir(callerFile)
		if strings.HasPrefix(qualifier, ".") {
			dots := len(qualifier) - len(strings.TrimLeft(qualifier, "."))
			base := callerDir
			for i := 1; i < dots; i++ {
				base = path.Dir(base)
			}
			rel := strings.ReplaceAll(strings.TrimLeft(qualifier, "."), ".", "/")
			return idx.firstPkg(path.Join(base, rel), path.Join(base, rel, "__init__"))
		}
		mod := strings.ReplaceAll(qualifier, ".", "/")
		if pkg, ok := idx.longestSuffixPkg(mod, true); ok {
			return pkg, true
		}
		return idx.longestSuffixPkg(mod+"/__init__", true)

	default:
		if strings.HasPrefix(qualifier, ".") {
			target := path.Join(path.Dir(callerFile), qualifier)
			switch path.Ext(target) {
			case ".ts", ".tsx", ".js", ".jsx":
				target = strings.TrimSuffix(target, path.Ext(target))
			}
			return idx.firstPkg(target, path.Join(target, "index"))
		}
		mod := strings.TrimPrefix(strings.TrimPrefix(qualifier, "@/"), "~/")
		if pkg, ok := idx.longestSuffixPkg(mod, true); ok {
			return pkg, true
		}
		return idx.longestSuffixPkg(mod+"/index", true)
	}
}

func (idx *symbolIndex) hasPkg(pkg string) (string, bool) {
	_, ok := idx.byPkg[pkg]
	return pkg, ok
}

func (idx *symbolIndex) firstPkg(candidates ...string) (string, bool) {
	for _, c := range candidates {
		if _, ok := idx.byPkg[path.Clean(c)]; ok {
			return path.Clean(c), true
		}
	}
	return "", false
}

// longestSuffixPkg 在仓库包中找与导入路径按路径段后缀对齐的最长匹配。
// Go 导入路径比仓库目录长（module/dir），只允许 importPath 以 pkg 结尾；
// Python/TS 模块说明符可能比仓库路径短（src/pkg/mod ← pkg.mod），allowShorter 时也允许 pkg 以 importPath 结尾。
func (idx *symbolIndex) longestSuffixPkg(importPath string, allowShorter bool) (string, bool) {
	best := ""
	for _, pkg := range idx.pkgs {
		if pkg == "." || pkg == "" {
			continue
		}
		if importPath == pkg || strings.HasSuffix(importPath, "/"+pkg) || (allowShorter && strings.HasSuffix(pkg, "/"+importPath)) {
			if len(pkg) > len(best) {
				best = pkg
			}
		}
	}
	return best, best != ""
}

// ReadGoModulePath 读取仓库根目录 go.mod 中的 module 路径，不存在时返回空串
func ReadGoModulePath(repoPath string) string {
	data, err := os.ReadFile(filepath.Join(repoPath, "go.mod"))
	if err != nil {
		return ""
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if rest, ok := strings.CutPrefix(line, "module"); ok && (rest == "" || rest[0] == ' ' || rest[0] == '\t') {
			return strings.Trim(strings.TrimSpace(rest), "\"")
		}
	}
	return ""
}
