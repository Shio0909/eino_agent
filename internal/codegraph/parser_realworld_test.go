package codegraph

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseDeerFlowReal(t *testing.T) {
	repoDir := filepath.Join("..", "..", "data", "test_repos", "deer-flow")
	if _, err := os.Stat(repoDir); os.IsNotExist(err) {
		t.Skip("deer-flow repo not found, skipping real test")
	}

	parser := NewParser()
	ctx := context.Background()

	var allResults []*ParseResult
	totalEntities := 0
	totalRelations := 0
	fileCount := 0
	start := time.Now()

	// 遍历 backend/ 下所有 Python 文件
	backendDir := filepath.Join(repoDir, "backend")
	err := filepath.Walk(backendDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		ext := filepath.Ext(path)
		lang, ok := DetectLanguage(ext)
		if !ok {
			return nil
		}

		source, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if len(source) == 0 {
			return nil
		}

		relPath, _ := filepath.Rel(repoDir, path)
		result, err := parser.Parse(ctx, relPath, source, lang)
		if err != nil {
			t.Logf("⚠️ Parse error %s: %v", relPath, err)
			return nil
		}

		allResults = append(allResults, result)
		totalEntities += len(result.Entities)
		totalRelations += len(result.Relations)
		fileCount++
		return nil
	})
	if err != nil {
		t.Fatalf("Walk failed: %v", err)
	}

	elapsed := time.Since(start)

	fmt.Printf("\n╔══════════════════════════════════════════════╗\n")
	fmt.Printf("║  deer-flow backend 真实代码解析结果          ║\n")
	fmt.Printf("╠══════════════════════════════════════════════╣\n")
	fmt.Printf("║  文件数: %-5d  耗时: %-20s  ║\n", fileCount, elapsed)
	fmt.Printf("║  实体数: %-5d  关系数: %-18d  ║\n", totalEntities, totalRelations)
	fmt.Printf("╚══════════════════════════════════════════════╝\n")

	// 按类型统计
	typeCounts := map[EntityType]int{}
	relCounts := map[RelationType]int{}
	for _, r := range allResults {
		for _, e := range r.Entities {
			typeCounts[e.Type]++
		}
		for _, rel := range r.Relations {
			relCounts[rel.Type]++
		}
	}

	fmt.Println("\n── 实体类型分布 ──")
	for tp, cnt := range typeCounts {
		fmt.Printf("  %-12s %d\n", tp, cnt)
	}

	fmt.Println("\n── 关系类型分布 ──")
	for tp, cnt := range relCounts {
		fmt.Printf("  %-12s %d\n", tp, cnt)
	}

	// 打印最大的文件（实体最多）
	fmt.Println("\n── Top 10 复杂文件（按实体数） ──")
	type fileStat struct {
		path      string
		entities  int
		relations int
	}
	var stats []fileStat
	for _, r := range allResults {
		stats = append(stats, fileStat{r.FilePath, len(r.Entities), len(r.Relations)})
	}
	// 简单排序
	for i := 0; i < len(stats); i++ {
		for j := i + 1; j < len(stats); j++ {
			if stats[j].entities > stats[i].entities {
				stats[i], stats[j] = stats[j], stats[i]
			}
		}
	}
	for i, s := range stats {
		if i >= 10 {
			break
		}
		fmt.Printf("  %3d entities, %3d relations: %s\n", s.entities, s.relations, s.path)
	}

	// 展示一些有趣的调用链
	fmt.Println("\n── 示例调用关系（前 20 条 CALLS） ──")
	count := 0
	for _, r := range allResults {
		for _, rel := range r.Relations {
			if rel.Type == RelCalls && count < 20 {
				fmt.Printf("  %s → %s\n", rel.Source, rel.Target)
				count++
			}
		}
	}

	// 展示继承关系
	fmt.Println("\n── 继承关系 ──")
	for _, r := range allResults {
		for _, rel := range r.Relations {
			if rel.Type == RelInherits {
				fmt.Printf("  %s extends %s\n", rel.Source, rel.Target)
			}
		}
	}

	// 也扫描前端 TS 文件
	fmt.Println("\n\n=== 前端 TypeScript 文件扫描 ===")
	frontendDir := filepath.Join(repoDir, "frontend")
	tsFileCount := 0
	tsEntities := 0
	tsRelations := 0

	_ = filepath.Walk(frontendDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		if strings.Contains(path, "node_modules") || strings.Contains(path, ".next") {
			return filepath.SkipDir
		}
		ext := filepath.Ext(path)
		lang, ok := DetectLanguage(ext)
		if !ok {
			return nil
		}
		source, _ := os.ReadFile(path)
		if len(source) == 0 {
			return nil
		}
		relPath, _ := filepath.Rel(repoDir, path)
		result, err := parser.Parse(ctx, relPath, source, lang)
		if err != nil {
			return nil
		}
		tsFileCount++
		tsEntities += len(result.Entities)
		tsRelations += len(result.Relations)
		return nil
	})

	fmt.Printf("  TS/JS 文件: %d | 实体: %d | 关系: %d\n", tsFileCount, tsEntities, tsRelations)

	// 基本断言
	if totalEntities < 10 {
		t.Errorf("Expected more entities from deer-flow backend, got %d", totalEntities)
	}
}
