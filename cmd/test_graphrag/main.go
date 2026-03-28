// GraphRAG 效果验证测试：测试 10 个不同类型的查询，检查回答质量和图谱贡献
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	baseURL = "http://127.0.0.1:19093"
	kbID    = "3971338d-649d-43c4-91b7-12f7543b7660"
)

type testCase struct {
	Question      string
	Category      string   // multi-hop / entity-comparison / single-doc
	ExpectedTerms []string // 回答中应出现的关键术语
}

type chatReq struct {
	Query           string `json:"query"`
	KnowledgeBaseID string `json:"knowledge_base_id"`
	Mode            string `json:"mode"`
}

type chatResp struct {
	Answer     string `json:"answer"`
	References []struct {
		ID      string  `json:"id"`
		Content string  `json:"content"`
		Score   float64 `json:"score"`
	} `json:"references"`
	LatencyMs int `json:"latency_ms"`
}

func query(q string) (*chatResp, error) {
	body, _ := json.Marshal(chatReq{
		Query:           q,
		KnowledgeBaseID: kbID,
		Mode:            "pipeline",
	})
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Post(baseURL+"/api/v1/chat", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(data[:minInt(200, len(data))]))
	}
	var r chatResp
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func countTerms(answer string, terms []string) (int, []string) {
	lower := strings.ToLower(answer)
	found := 0
	var missing []string
	for _, t := range terms {
		if strings.Contains(lower, strings.ToLower(t)) {
			found++
		} else {
			missing = append(missing, t)
		}
	}
	return found, missing
}

func main() {
	cases := []testCase{
		{
			Question:      "Kubernetes Deployment 和 Service 是如何协同工作来暴露应用的？",
			Category:      "multi-hop",
			ExpectedTerms: []string{"Deployment", "Pod", "Service", "selector", "replica"},
		},
		{
			Question:      "Pod 的 readiness probe 和 liveness probe 分别起什么作用？",
			Category:      "multi-hop",
			ExpectedTerms: []string{"readiness", "liveness", "Pod", "restart", "traffic"},
		},
		{
			Question:      "Go module 的 go.mod 和 go.sum 文件各自的作用是什么？",
			Category:      "multi-hop",
			ExpectedTerms: []string{"go.mod", "go.sum", "module", "require"},
		},
		{
			Question:      "Kubernetes 中 Deployment 的滚动更新策略如何保证服务不中断？",
			Category:      "multi-hop",
			ExpectedTerms: []string{"maxSurge", "maxUnavailable", "Pod", "update"},
		},
		{
			Question:      "Go 语言中如何使用 go test 运行测试？测试文件的命名规则是什么？",
			Category:      "single-doc",
			ExpectedTerms: []string{"_test.go", "Test", "testing", "go test"},
		},
		{
			Question:      "Kubernetes Service 的 ClusterIP、NodePort 和 LoadBalancer 有什么区别？",
			Category:      "entity-comparison",
			ExpectedTerms: []string{"ClusterIP", "NodePort", "LoadBalancer"},
		},
		{
			Question:      "Go 中 goroutine 和 channel 如何配合实现并发？",
			Category:      "multi-hop",
			ExpectedTerms: []string{"goroutine", "channel"},
		},
		{
			Question:      "Kubernetes Pod 中 init container 和普通 container 有什么区别？",
			Category:      "entity-comparison",
			ExpectedTerms: []string{"init", "container", "Pod"},
		},
		{
			Question:      "Go module 中 replace 和 require 指令分别用于什么场景？",
			Category:      "entity-comparison",
			ExpectedTerms: []string{"replace", "require", "module"},
		},
		{
			Question:      "Kubernetes Deployment 的 replicas、selector 和 template 三个字段如何关联？",
			Category:      "multi-hop",
			ExpectedTerms: []string{"replicas", "selector", "template", "Pod"},
		},
	}

	fmt.Println("================================================================")
	fmt.Println("  GraphRAG 效果验证测试 (10 queries)")
	fmt.Printf("  KB: %s\n", kbID)
	fmt.Printf("  Server: %s\n", baseURL)
	fmt.Println("================================================================")

	var totalLatency int
	var totalTermHit, totalTerms int
	var passed, failed int

	for i, tc := range cases {
		fmt.Printf("\n[%d/%d] <%s> %s\n", i+1, len(cases), tc.Category, tc.Question)

		start := time.Now()
		resp, err := query(tc.Question)
		elapsed := time.Since(start)

		if err != nil {
			fmt.Printf("  X 请求失败: %v\n", err)
			failed++
			continue
		}

		termHit, missing := countTerms(resp.Answer, tc.ExpectedTerms)
		coverage := float64(termHit) / float64(len(tc.ExpectedTerms)) * 100

		totalLatency += int(elapsed.Milliseconds())
		totalTermHit += termHit
		totalTerms += len(tc.ExpectedTerms)

		hasGraphCtx := false
		for _, ref := range resp.References {
			if ref.ID == "graph-context" {
				hasGraphCtx = true
				break
			}
		}

		if coverage >= 60 {
			fmt.Printf("  PASS 术语覆盖: %d/%d (%.0f%%)", termHit, len(tc.ExpectedTerms), coverage)
			passed++
		} else {
			fmt.Printf("  WARN 术语覆盖: %d/%d (%.0f%%)", termHit, len(tc.ExpectedTerms), coverage)
			failed++
		}
		fmt.Printf("  延迟: %v  引用: %d  图谱上下文: %v\n", elapsed.Round(time.Millisecond), len(resp.References), hasGraphCtx)

		if len(missing) > 0 {
			fmt.Printf("       缺失: %v\n", missing)
		}
		ansRunes := []rune(resp.Answer)
		if len(ansRunes) > 120 {
			ansRunes = ansRunes[:120]
		}
		fmt.Printf("       回答: %s...\n", string(ansRunes))
	}

	total := passed + failed
	fmt.Println("\n================================================================")
	fmt.Println("  测试结果汇总")
	fmt.Println("================================================================")
	if total > 0 {
		fmt.Printf("  通过/总数: %d/%d (%.0f%%)\n", passed, total, float64(passed)/float64(total)*100)
		fmt.Printf("  术语覆盖: %d/%d (%.0f%%)\n", totalTermHit, totalTerms, float64(totalTermHit)/float64(totalTerms)*100)
		fmt.Printf("  平均延迟: %dms\n", totalLatency/total)
	}
	fmt.Println("================================================================")
}
