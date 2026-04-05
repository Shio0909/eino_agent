package codegraph

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Indexer 代码仓库索引编排器
// 遍历文件 → AST 解析 → 写入 Neo4j
type Indexer struct {
	parser     *Parser
	repo       CodeGraphRepository
	reposDir   string
	maxWorkers int
}

// IndexProgress 索引进度
type IndexProgress struct {
	Repo        string `json:"repo"`
	TotalFiles  int    `json:"total_files"`
	Processed   int    `json:"processed"`
	Entities    int    `json:"entities"`
	Relations   int    `json:"relations"`
	Errors      int    `json:"errors"`
	ElapsedMs   int64  `json:"elapsed_ms"`
}

// NewIndexer 创建索引编排器
func NewIndexer(repo CodeGraphRepository, reposDir string) *Indexer {
	return &Indexer{
		parser:     NewParser(),
		repo:       repo,
		reposDir:   reposDir,
		maxWorkers: 4,
	}
}

// IndexRepo 对仓库进行全量索引
func (idx *Indexer) IndexRepo(ctx context.Context, repoName string) (*IndexProgress, error) {
	repoPath := filepath.Join(idx.reposDir, repoName)
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("repo not found: %s", repoPath)
	}

	start := time.Now()
	log.Printf("[indexer] starting full index for %s", repoName)

	// 收集所有可解析文件
	var files []string
	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		// 跳过隐藏目录和常见排除目录
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "__pycache__" ||
				name == "vendor" || name == ".venv" || name == "dist" || name == "build" {
				return filepath.SkipDir
			}
			return nil
		}
		ext := filepath.Ext(path)
		if _, ok := DetectLanguage(ext); ok {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk repo: %w", err)
	}

	log.Printf("[indexer] found %d parseable files in %s", len(files), repoName)

	progress := &IndexProgress{
		Repo:       repoName,
		TotalFiles: len(files),
	}

	// 并发解析
	var (
		processed int64
		entities  int64
		relations int64
		errors    int64
		wg        sync.WaitGroup
		sem       = make(chan struct{}, idx.maxWorkers)
	)

	for _, file := range files {
		wg.Add(1)
		sem <- struct{}{}
		go func(filePath string) {
			defer wg.Done()
			defer func() { <-sem }()

			relPath, _ := filepath.Rel(repoPath, filePath)
			relPath = filepath.ToSlash(relPath) // 统一用斜杠

			ext := filepath.Ext(filePath)
			lang, _ := DetectLanguage(ext)

			source, err := os.ReadFile(filePath)
			if err != nil || len(source) == 0 {
				atomic.AddInt64(&processed, 1)
				return
			}

			// 计算文件 hash
			hash := fmt.Sprintf("%x", sha256.Sum256(source))

			// 解析
			result, err := idx.parser.Parse(ctx, relPath, source, lang)
			if err != nil {
				log.Printf("[indexer] parse error %s: %v", relPath, err)
				atomic.AddInt64(&errors, 1)
				atomic.AddInt64(&processed, 1)
				return
			}

			// 写入 Neo4j
			if err := idx.repo.UpsertFile(ctx, repoName, relPath, hash); err != nil {
				log.Printf("[indexer] upsert file error %s: %v", relPath, err)
			}
			if err := idx.repo.UpsertEntities(ctx, repoName, result.Entities); err != nil {
				log.Printf("[indexer] upsert entities error %s: %v", relPath, err)
			}
			if err := idx.repo.UpsertRelations(ctx, repoName, result.Relations); err != nil {
				log.Printf("[indexer] upsert relations error %s: %v", relPath, err)
			}

			atomic.AddInt64(&entities, int64(len(result.Entities)))
			atomic.AddInt64(&relations, int64(len(result.Relations)))
			atomic.AddInt64(&processed, 1)
		}(file)
	}

	wg.Wait()

	progress.Processed = int(processed)
	progress.Entities = int(entities)
	progress.Relations = int(relations)
	progress.Errors = int(errors)
	progress.ElapsedMs = time.Since(start).Milliseconds()

	log.Printf("[indexer] completed %s: %d files, %d entities, %d relations, %d errors in %dms",
		repoName, progress.Processed, progress.Entities, progress.Relations, progress.Errors, progress.ElapsedMs)

	return progress, nil
}

// IncrementalUpdate 增量更新：只重建变更的文件
func (idx *Indexer) IncrementalUpdate(ctx context.Context, repoName string) (*IndexProgress, error) {
	repoPath := filepath.Join(idx.reposDir, repoName)

	// 获取变更文件列表
	changedFiles, err := idx.getChangedFiles(ctx, repoPath)
	if err != nil {
		log.Printf("[indexer] cannot detect changes for %s, falling back to full index: %v", repoName, err)
		return idx.IndexRepo(ctx, repoName)
	}

	if len(changedFiles) == 0 {
		log.Printf("[indexer] no changes detected for %s", repoName)
		return &IndexProgress{Repo: repoName, TotalFiles: 0}, nil
	}

	start := time.Now()
	log.Printf("[indexer] incremental update for %s: %d changed files", repoName, len(changedFiles))

	progress := &IndexProgress{
		Repo:       repoName,
		TotalFiles: len(changedFiles),
	}

	for _, relPath := range changedFiles {
		fullPath := filepath.Join(repoPath, relPath)

		// 先删除旧的图数据
		if err := idx.repo.DeleteFileGraph(ctx, repoName, relPath); err != nil {
			log.Printf("[indexer] delete old graph error %s: %v", relPath, err)
		}

		// 检查文件是否还存在（可能是删除操作）
		source, err := os.ReadFile(fullPath)
		if err != nil {
			progress.Processed++
			continue // 文件被删除，已经清理了图数据
		}

		ext := filepath.Ext(fullPath)
		lang, ok := DetectLanguage(ext)
		if !ok {
			progress.Processed++
			continue
		}

		hash := fmt.Sprintf("%x", sha256.Sum256(source))

		result, err := idx.parser.Parse(ctx, relPath, source, lang)
		if err != nil {
			progress.Errors++
			progress.Processed++
			continue
		}

		_ = idx.repo.UpsertFile(ctx, repoName, relPath, hash)
		_ = idx.repo.UpsertEntities(ctx, repoName, result.Entities)
		_ = idx.repo.UpsertRelations(ctx, repoName, result.Relations)

		progress.Entities += len(result.Entities)
		progress.Relations += len(result.Relations)
		progress.Processed++
	}

	progress.ElapsedMs = time.Since(start).Milliseconds()
	log.Printf("[indexer] incremental update completed: %d files, %d entities, %d relations in %dms",
		progress.Processed, progress.Entities, progress.Relations, progress.ElapsedMs)

	return progress, nil
}

// getChangedFiles 通过 git diff 获取变更文件列表
func (idx *Indexer) getChangedFiles(ctx context.Context, repoPath string) ([]string, error) {
	// 获取上次 fetch 以来的变更
	cmd := exec.CommandContext(ctx, "git", "diff", "--name-only", "HEAD~1..HEAD")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var changed []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// 只关注我们支持的语言
		ext := filepath.Ext(line)
		if _, ok := DetectLanguage(ext); ok {
			changed = append(changed, filepath.ToSlash(line))
		}
	}
	return changed, nil
}
