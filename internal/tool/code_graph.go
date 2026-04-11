package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"eino_agent/internal/codegraph"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// CodeGraphTool 代码知识图谱查询工具
// 通过 Neo4j 知识图谱查询代码的调用链、依赖关系、定义等
type CodeGraphTool struct {
	repo    codegraph.CodeGraphRepository
	indexer *codegraph.Indexer
}

type codeGraphInput struct {
	Action string `json:"action"`          // call_chain, dependencies, definition, structure, search, overview, index
	Repo   string `json:"repo"`            // 仓库名
	Name   string `json:"name,omitempty"`  // 符号名称
	Path   string `json:"path,omitempty"`  // 文件路径
	Depth  int    `json:"depth,omitempty"` // 查询深度
}

type codeGraphOutput struct {
	Action   string                  `json:"action"`
	Results  []codeGraphResult       `json:"results,omitempty"`
	Overview *codegraph.RepoOverview `json:"overview,omitempty"`
	Progress *codegraph.IndexProgress `json:"progress,omitempty"`
	Summary  string                  `json:"summary"`
}

type codeGraphResult struct {
	Name      string `json:"name"`
	Type      string `json:"type,omitempty"`
	FilePath  string `json:"file_path,omitempty"`
	LineStart int    `json:"line_start,omitempty"`
	LineEnd   int    `json:"line_end,omitempty"`
	Relation  string `json:"relation,omitempty"`
	Target    string `json:"target,omitempty"`
}

func NewCodeGraphTool(repo codegraph.CodeGraphRepository, indexer *codegraph.Indexer) *CodeGraphTool {
	return &CodeGraphTool{repo: repo, indexer: indexer}
}

func (t *CodeGraphTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "code_graph",
		Desc: `查询代码知识图谱（基于 AST 解析 + Neo4j 存储）。支持以下操作：
- index: 对仓库建立代码知识图谱索引（首次使用前必须执行）
- call_chain: 查找函数的调用链（谁调用了它，它调用了谁）
- definition: 查找符号定义（函数、类、方法的源码位置）
- structure: 查看某文件定义了哪些函数/类/方法
- search: 模糊搜索符号名称
- overview: 获取仓库图谱统计信息
使用前需先通过 repo_manager 克隆仓库，再用 index 建索引。`,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"action": {
				Type:     schema.String,
				Desc:     "操作: index(建索引), call_chain(调用链), definition(查定义), structure(文件结构), search(搜索符号), overview(仓库概览)",
				Required: true,
			},
			"repo": {
				Type:     schema.String,
				Desc:     "仓库名称，如 deer-flow",
				Required: true,
			},
			"name": {
				Type: schema.String,
				Desc: "要查询的符号名称（call_chain/definition/search 时使用）",
			},
			"path": {
				Type: schema.String,
				Desc: "文件路径（structure 时使用），如 backend/app/gateway/app.py",
			},
			"depth": {
				Type: schema.Integer,
				Desc: "调用链查询深度（1-5，默认 2）",
			},
		}),
	}, nil
}

func (t *CodeGraphTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var params codeGraphInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
		return "", fmt.Errorf("parse params: %w", err)
	}

	if params.Repo == "" {
		return "", fmt.Errorf("repo is required")
	}

	var output codeGraphOutput
	var err error

	switch strings.ToLower(params.Action) {
	case "index":
		output, err = t.doIndex(ctx, params)
	case "call_chain":
		output, err = t.doCallChain(ctx, params)
	case "definition":
		output, err = t.doDefinition(ctx, params)
	case "structure":
		output, err = t.doStructure(ctx, params)
	case "search":
		output, err = t.doSearch(ctx, params)
	case "overview":
		output, err = t.doOverview(ctx, params)
	default:
		return "", fmt.Errorf("unknown action: %s", params.Action)
	}

	if err != nil {
		output = codeGraphOutput{
			Action:  params.Action,
			Summary: fmt.Sprintf("Error: %v", err),
		}
	}

	data, _ := json.Marshal(output)
	// 截断过大输出
	result := string(data)
	if len(result) > 4000 {
		result = result[:4000] + "...(truncated)"
	}
	return result, nil
}

func (t *CodeGraphTool) doIndex(ctx context.Context, params codeGraphInput) (codeGraphOutput, error) {
	if t.indexer == nil {
		return codeGraphOutput{}, fmt.Errorf("indexer not configured")
	}

	progress, err := t.indexer.IndexRepo(ctx, params.Repo)
	if err != nil {
		return codeGraphOutput{}, err
	}

	return codeGraphOutput{
		Action:   "index",
		Progress: progress,
		Summary: fmt.Sprintf("Indexed %s: %d files, %d entities, %d relations in %dms",
			params.Repo, progress.Processed, progress.Entities, progress.Relations, progress.ElapsedMs),
	}, nil
}

func (t *CodeGraphTool) doCallChain(ctx context.Context, params codeGraphInput) (codeGraphOutput, error) {
	if params.Name == "" {
		return codeGraphOutput{}, fmt.Errorf("name is required for call_chain")
	}

	depth := params.Depth
	if depth <= 0 {
		depth = 2
	}

	// 获取调用者和被调用者
	callers, err := t.repo.FindCallers(ctx, params.Repo, params.Name, depth)
	if err != nil {
		return codeGraphOutput{}, err
	}

	callees, err := t.repo.FindCallees(ctx, params.Repo, params.Name, depth)
	if err != nil {
		return codeGraphOutput{}, err
	}

	var results []codeGraphResult
	for _, r := range callers {
		results = append(results, codeGraphResult{
			Name:     r.Source,
			Relation: "CALLS",
			Target:   r.Target,
		})
	}
	for _, r := range callees {
		results = append(results, codeGraphResult{
			Name:     r.Source,
			Relation: "CALLS",
			Target:   r.Target,
		})
	}

	return codeGraphOutput{
		Action:  "call_chain",
		Results: results,
		Summary: fmt.Sprintf("Call chain for '%s': %d callers, %d callees (depth=%d)",
			params.Name, len(callers), len(callees), depth),
	}, nil
}

func (t *CodeGraphTool) doDefinition(ctx context.Context, params codeGraphInput) (codeGraphOutput, error) {
	if params.Name == "" {
		return codeGraphOutput{}, fmt.Errorf("name is required for definition")
	}

	entities, err := t.repo.FindDefinition(ctx, params.Repo, params.Name)
	if err != nil {
		return codeGraphOutput{}, err
	}

	var results []codeGraphResult
	for _, e := range entities {
		results = append(results, codeGraphResult{
			Name:      e.QualifiedName,
			Type:      string(e.Type),
			FilePath:  e.FilePath,
			LineStart: e.LineStart,
			LineEnd:   e.LineEnd,
		})
	}

	return codeGraphOutput{
		Action:  "definition",
		Results: results,
		Summary: fmt.Sprintf("Found %d definitions for '%s'", len(results), params.Name),
	}, nil
}

func (t *CodeGraphTool) doStructure(ctx context.Context, params codeGraphInput) (codeGraphOutput, error) {
	if params.Path == "" {
		return codeGraphOutput{}, fmt.Errorf("path is required for structure")
	}

	entities, err := t.repo.GetFileStructure(ctx, params.Repo, params.Path)
	if err != nil {
		return codeGraphOutput{}, err
	}

	var results []codeGraphResult
	for _, e := range entities {
		results = append(results, codeGraphResult{
			Name:      e.QualifiedName,
			Type:      string(e.Type),
			LineStart: e.LineStart,
			LineEnd:   e.LineEnd,
		})
	}

	return codeGraphOutput{
		Action:  "structure",
		Results: results,
		Summary: fmt.Sprintf("File %s: %d entities", params.Path, len(results)),
	}, nil
}

func (t *CodeGraphTool) doSearch(ctx context.Context, params codeGraphInput) (codeGraphOutput, error) {
	if params.Name == "" {
		return codeGraphOutput{}, fmt.Errorf("name is required for search")
	}

	entities, err := t.repo.SearchSymbol(ctx, params.Repo, params.Name, 20)
	if err != nil {
		return codeGraphOutput{}, err
	}

	var results []codeGraphResult
	for _, e := range entities {
		results = append(results, codeGraphResult{
			Name:      e.QualifiedName,
			Type:      string(e.Type),
			FilePath:  e.FilePath,
			LineStart: e.LineStart,
		})
	}

	return codeGraphOutput{
		Action:  "search",
		Results: results,
		Summary: fmt.Sprintf("Found %d symbols matching '%s'", len(results), params.Name),
	}, nil
}

func (t *CodeGraphTool) doOverview(ctx context.Context, params codeGraphInput) (codeGraphOutput, error) {
	overview, err := t.repo.GetRepoOverview(ctx, params.Repo)
	if err != nil {
		return codeGraphOutput{}, err
	}

	return codeGraphOutput{
		Action:   "overview",
		Overview: overview,
		Summary: fmt.Sprintf("Repo %s: %d files, %d entities, %d relations",
			params.Repo, overview.FileCount, overview.EntityCount, overview.RelationCount),
	}, nil
}
