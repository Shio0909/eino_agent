// Package graphrag - 图抽取器
//
// 【核心流程】文档 Chunk → LLM 抽取实体和关系 → 解析 JSON → 存入 Neo4j
//
// 参考 WeKnora:
// - internal/application/service/chat_pipline/extract_entity.go (Extractor/Formater)
// - internal/application/service/graph.go (BuildGraph 批量抽取)
package graphrag

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// ── Prompt 模板 ──

// 实体和关系抽取 Prompt（参考 WeKnora config/config.yaml extract_graph.description）
const extractGraphPrompt = `请基于给定文本，按以下步骤完成信息提取任务，确保逻辑清晰、信息完整准确：

## 一、实体提取与属性补充
1. **提取核心实体**：通读文本，按逻辑顺序提取所有核心实体（人物、组织、概念、技术、地点等）。
2. **标注实体类型**：为每个实体标注类型，必须是以下之一：
   - Technology: 技术、框架、工具、编程语言、数据库
   - Concept: 抽象概念、方法论、设计模式、算法
   - Component: 组件、模块、服务、子系统
   - API: 接口、方法、协议、端点
   - Person: 人物
   - Organization: 组织、公司、社区
   - Other: 不属于以上类型的实体
3. **补充实体属性**：针对每个实体，补充文本中明确提及的属性信息。

## 二、关系提取与验证
1. **提取有效关系**：基于已提取的实体，识别文本中真实存在的关系，确保关系符合文本事实。
2. **明确关系类型**：使用简洁的关系类型标签，如"作者"、"属于"、"包含"、"使用"等。
3. **明确关系主体**：对每组关系，清晰标注两个关联实体。

## 输出格式
以 JSON 数组格式输出，每个元素为以下两种之一：

实体格式：
{"entity": "实体名称", "entity_type": "类型", "entity_attributes": ["属性1", "属性2"]}

关系格式：
{"entity1": "源实体", "entity2": "目标实体", "relation": "关系类型"}

## 示例

Q: Kubernetes 使用 etcd 作为分布式存储，kube-dns 组件提供集群内服务发现功能。

A: ` + "```json" + `
[
  {"entity": "Kubernetes", "entity_type": "Technology", "entity_attributes": ["容器编排平台"]},
  {"entity": "etcd", "entity_type": "Component", "entity_attributes": ["分布式键值存储"]},
  {"entity": "kube-dns", "entity_type": "Component", "entity_attributes": ["DNS 服务"]},
  {"entity": "服务发现", "entity_type": "Concept", "entity_attributes": ["集群内服务定位机制"]},
  {"entity1": "Kubernetes", "entity2": "etcd", "relation": "使用"},
  {"entity1": "kube-dns", "entity2": "服务发现", "relation": "提供"}
]
` + "```" + `

Q: 《红楼梦》，又名《石头记》，是清代作家曹雪芹创作的中国古典四大名著之一。

A: ` + "```json" + `
[
  {"entity": "红楼梦", "entity_type": "Other", "entity_attributes": ["中国古典四大名著之一", "又名《石头记》"]},
  {"entity": "石头记", "entity_type": "Other", "entity_attributes": ["《红楼梦》的别名"]},
  {"entity": "曹雪芹", "entity_type": "Person", "entity_attributes": ["清代作家"]},
  {"entity1": "红楼梦", "entity2": "曹雪芹", "relation": "作者"},
  {"entity1": "红楼梦", "entity2": "石头记", "relation": "别名"}
]
` + "```"

// 查询时实体抽取 Prompt（参考 WeKnora extract_entity.description）
const extractEntityPrompt = `请基于用户给的问题，提取其中的关键实体用于知识图谱检索：

## 规则
1. **提取原子实体**：每个实体应该是单一概念，不要组合多个词。例如 "Kubernetes Service" 应拆分为 "Kubernetes" 和 "Service" 两个实体。
2. **标注实体类型**：为每个实体标注类型（Technology/Concept/Component/API/Person/Organization/Other）。
3. **双语提取**：如果问题包含中文术语，同时提取其英文等价形式（反之亦然）。例如 "服务发现" → 同时输出 "服务发现" 和 "Service Discovery"。
4. **按关联度排序**：核心实体在前，辅助实体在后。
5. **去除动词和修饰词**：只保留名词性实体。

## 输出格式
以 JSON 数组格式输出实体列表：

` + "```json" + `
[
  {"entity": "实体1", "entity_type": "类型"},
  {"entity": "实体2", "entity_type": "类型"}
]
` + "```" + `

## 示例

Q: Kubernetes Service 如何实现服务发现？
A: ` + "```json" + `
[
  {"entity": "Kubernetes", "entity_type": "Technology"},
  {"entity": "Service", "entity_type": "Component"},
  {"entity": "服务发现", "entity_type": "Concept"},
  {"entity": "Service Discovery", "entity_type": "Concept"}
]
` + "```" + `

Q: 《红楼梦》的作者是谁？
A: ` + "```json" + `
[
  {"entity": "红楼梦", "entity_type": "Other"},
  {"entity": "作者", "entity_type": "Other"}
]
` + "```"

// ── Extractor 实体/关系抽取器 ──

// Extractor 从文本中抽取实体和关系
type Extractor struct {
	chatModel        model.ChatModel // 重模型 — 用于 BuildGraph 时的完整实体/关系抽取
	lightModel       model.ChatModel // 轻量模型 — 用于查询时的快速实体抽取（可选，为 nil 则退化到 chatModel）
	useLightForBuild bool            // 是否用轻量模型替代重模型进行建图（牺牲质量换速度）
	temp             float64
}

// NewExtractor 创建抽取器
func NewExtractor(chatModel model.ChatModel, temperature float64) *Extractor {
	if temperature <= 0 {
		temperature = 0.1 // 低温度保证抽取一致性
	}
	return &Extractor{chatModel: chatModel, temp: temperature}
}

// SetLightModel 注入轻量模型，用于查询时实体抽取（延迟敏感场景）
func (e *Extractor) SetLightModel(m model.ChatModel) {
	e.lightModel = m
}

// SetUseLightForBuild 启用轻量模型建图（速度快但抽取质量可能略降）
func (e *Extractor) SetUseLightForBuild(use bool) {
	e.useLightForBuild = use
}

// queryModel 返回查询时使用的模型（优先轻量模型）
func (e *Extractor) queryModel() model.ChatModel {
	if e.lightModel != nil {
		return e.lightModel
	}
	return e.chatModel
}

// buildModel 返回建图时使用的模型
func (e *Extractor) buildModel() model.ChatModel {
	if e.useLightForBuild && e.lightModel != nil {
		return e.lightModel
	}
	return e.chatModel
}

// ExtractFromChunk 从文档 Chunk 中抽取实体和关系
func (e *Extractor) ExtractFromChunk(ctx context.Context, content string) (*GraphData, error) {
	if content == "" {
		return &GraphData{}, nil
	}

	messages := []*schema.Message{
		{Role: schema.System, Content: extractGraphPrompt},
		{Role: schema.User, Content: content},
	}

	resp, err := e.buildModel().Generate(ctx, messages,
		model.WithTemperature(float32(e.temp)),
	)
	if err != nil {
		return nil, fmt.Errorf("LLM 实体抽取失败: %w", err)
	}

	graph, err := ParseGraphJSON(resp.Content)
	if err != nil {
		log.Printf("[GraphRAG] 解析抽取结果失败: %v, 原始响应: %s", err, truncateStr(resp.Content, 200))
		return &GraphData{}, nil // 解析失败不中断流程
	}

	return graph, nil
}

// ExtractEntitiesFromQuery 从用户查询中抽取实体列表（含类型信息）
// 使用轻量模型（如果已配置），因为查询时实体抽取对延迟敏感
func (e *Extractor) ExtractEntitiesFromQuery(ctx context.Context, query string) ([]QueryEntity, error) {
	messages := []*schema.Message{
		{Role: schema.System, Content: extractEntityPrompt},
		{Role: schema.User, Content: fmt.Sprintf("# Question\nQ: %s\nA: ", query)},
	}

	resp, err := e.queryModel().Generate(ctx, messages,
		model.WithTemperature(float32(e.temp)),
	)
	if err != nil {
		return nil, fmt.Errorf("LLM 实体抽取失败: %w", err)
	}

	raw := parseQueryEntities(resp.Content)
	// 过滤太短的实体（单字符容易过度匹配，如 "n"、"r"）
	filtered := make([]QueryEntity, 0, len(raw))
	for _, entity := range raw {
		name := strings.TrimSpace(entity.Name)
		if len([]rune(name)) >= 2 {
			entity.Name = name
			filtered = append(filtered, entity)
		}
	}
	return filtered, nil
}

// ── JSON 解析器 ──
// 参考 WeKnora: Formater.ParseGraph / Formater.parseOutput

var fenceRE = regexp.MustCompile("```(?:[A-Za-z0-9_+-]+)?\\s*\\n?([\\s\\S]*?)```")

// ParseGraphJSON 解析 LLM 返回的 JSON 抽取结果
func ParseGraphJSON(text string) (*GraphData, error) {
	content := extractJSONContent(text)
	if content == "" {
		return nil, fmt.Errorf("未找到有效 JSON 内容")
	}

	var items []map[string]interface{}
	if err := json.Unmarshal([]byte(content), &items); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %w", err)
	}

	var nodes []*GraphNode
	var relations []*GraphRelation

	for _, item := range items {
		if entity, ok := item["entity"]; ok {
			// 实体
			attrs := extractStringArray(item["entity_attributes"])
			entityType := "Other"
			if t, ok := item["entity_type"]; ok {
				entityType = fmt.Sprintf("%v", t)
			}
			nodes = append(nodes, &GraphNode{
				Name:       fmt.Sprintf("%v", entity),
				Type:       entityType,
				Attributes: attrs,
			})
		} else if e1, ok1 := item["entity1"]; ok1 {
			if e2, ok2 := item["entity2"]; ok2 {
				// 关系
				relType := "RELATED_TO"
				if r, ok := item["relation"]; ok {
					relType = fmt.Sprintf("%v", r)
				}
				relations = append(relations, &GraphRelation{
					Node1: fmt.Sprintf("%v", e1),
					Node2: fmt.Sprintf("%v", e2),
					Type:  relType,
				})
			}
		}
	}

	graph := &GraphData{Node: nodes, Relation: relations}
	rebuildGraph(graph)
	return graph, nil
}

// parseQueryEntities 从 LLM 响应中解析实体列表（含类型）
func parseQueryEntities(text string) []QueryEntity {
	content := extractJSONContent(text)
	if content == "" {
		return nil
	}

	var items []map[string]interface{}
	if err := json.Unmarshal([]byte(content), &items); err != nil {
		return nil
	}

	var entities []QueryEntity
	for _, item := range items {
		name := ""
		if n, ok := item["entity"]; ok {
			name = fmt.Sprintf("%v", n)
		} else if n, ok := item["name"]; ok {
			name = fmt.Sprintf("%v", n)
		}
		if name == "" {
			continue
		}
		entityType := ""
		if t, ok := item["entity_type"]; ok {
			entityType = fmt.Sprintf("%v", t)
		}
		entities = append(entities, QueryEntity{Name: name, Type: entityType})
	}
	return entities
}

// extractJSONContent 从可能带 code fence 的文本中提取 JSON 内容
func extractJSONContent(text string) string {
	// 尝试从 ```json ... ``` 中提取
	matches := fenceRE.FindAllStringSubmatch(text, -1)
	if len(matches) > 0 {
		return strings.TrimSpace(matches[0][1])
	}

	// 尝试直接解析
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "[") || strings.HasPrefix(text, "{") {
		return text
	}

	return ""
}

// rebuildGraph 清理图数据：去重节点、修复缺失节点
// 参考 WeKnora: Formater.rebuildGraph
func rebuildGraph(graph *GraphData) {
	nodeMap := make(map[string]*GraphNode)
	nodes := make([]*GraphNode, 0, len(graph.Node))
	for _, node := range graph.Node {
		if existing, ok := nodeMap[node.Name]; ok {
			// 合并属性
			existing.Attributes = append(existing.Attributes, node.Attributes...)
			continue
		}
		nodeMap[node.Name] = node
		nodes = append(nodes, node)
	}

	relations := make([]*GraphRelation, 0, len(graph.Relation))
	for _, rel := range graph.Relation {
		if rel.Node1 == rel.Node2 {
			continue // 跳过自环
		}
		// 确保关系的两端节点存在
		if _, ok := nodeMap[rel.Node1]; !ok {
			n := &GraphNode{Name: rel.Node1}
			nodes = append(nodes, n)
			nodeMap[rel.Node1] = n
		}
		if _, ok := nodeMap[rel.Node2]; !ok {
			n := &GraphNode{Name: rel.Node2}
			nodes = append(nodes, n)
			nodeMap[rel.Node2] = n
		}
		relations = append(relations, rel)
	}

	graph.Node = nodes
	graph.Relation = relations
}

// extractStringArray 从 interface{} 中提取字符串数组
func extractStringArray(v interface{}) []string {
	if v == nil {
		return nil
	}
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	result := make([]string, 0, len(arr))
	for _, item := range arr {
		result = append(result, fmt.Sprintf("%v", item))
	}
	return result
}

func truncateStr(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
