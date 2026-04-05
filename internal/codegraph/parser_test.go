package codegraph

import (
	"context"
	"fmt"
	"testing"
)

func TestParsePython(t *testing.T) {
	source := []byte(`
import os
from pathlib import Path

class BaseAgent:
    """基础 Agent"""
    def __init__(self, name):
        self.name = name

    def run(self, task):
        result = self.process(task)
        return result

class LeadAgent(BaseAgent):
    """主编排 Agent"""
    def __init__(self, name, model):
        super().__init__(name)
        self.model = model

    def process(self, task):
        subtasks = self.decompose(task)
        results = []
        for st in subtasks:
            r = self.execute(st)
            results.append(r)
        return self.merge(results)

    def decompose(self, task):
        return task.split(",")

    def execute(self, subtask):
        return subtask.strip()

    def merge(self, results):
        return " | ".join(results)

def create_agent(config):
    name = config.get("name", "default")
    model = config.get("model", "gpt-4")
    return LeadAgent(name, model)

def main():
    agent = create_agent({"name": "test", "model": "gpt-4"})
    result = agent.run("task1, task2, task3")
    print(result)
`)

	parser := NewParser()
	result, err := parser.Parse(context.Background(), "agent.py", source, LangPython)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	fmt.Printf("\n=== Python 解析结果 ===\n")
	fmt.Printf("文件: %s | 语言: %s\n", result.FilePath, result.Language)
	fmt.Printf("实体数: %d | 关系数: %d\n\n", len(result.Entities), len(result.Relations))

	fmt.Println("── 实体 ──")
	for _, e := range result.Entities {
		fmt.Printf("  [%s] %s (L%d-%d)", e.Type, e.QualifiedName, e.LineStart, e.LineEnd)
		if e.ClassName != "" {
			fmt.Printf(" ∈ %s", e.ClassName)
		}
		if e.Params != "" {
			fmt.Printf(" params=%s", e.Params)
		}
		fmt.Println()
	}

	fmt.Println("\n── 关系 ──")
	for _, r := range result.Relations {
		fmt.Printf("  %s -[%s]-> %s\n", r.Source, r.Type, r.Target)
	}

	// 验证
	if len(result.Entities) < 8 {
		t.Errorf("Expected at least 8 entities, got %d", len(result.Entities))
	}

	// 检查关键实体
	found := map[string]bool{}
	for _, e := range result.Entities {
		found[e.QualifiedName] = true
	}
	for _, expected := range []string{"BaseAgent", "LeadAgent", "create_agent", "main", "BaseAgent.run"} {
		if !found[expected] {
			t.Errorf("Missing entity: %s", expected)
		}
	}

	// 检查继承关系
	hasInherit := false
	for _, r := range result.Relations {
		if r.Type == RelInherits && r.Source == "LeadAgent" && r.Target == "BaseAgent" {
			hasInherit = true
		}
	}
	if !hasInherit {
		t.Error("Missing: LeadAgent -[INHERITS]-> BaseAgent")
	}
}

func TestParseGo(t *testing.T) {
	source := []byte(`package main

import (
	"context"
	"fmt"
	"log"
)

type Server struct {
	port int
	name string
}

type Handler interface {
	Handle(ctx context.Context) error
}

func NewServer(port int) *Server {
	return &Server{port: port, name: "default"}
}

func (s *Server) Start(ctx context.Context) error {
	log.Printf("Starting %s on port %d", s.name, s.port)
	return s.listen(ctx)
}

func (s *Server) listen(ctx context.Context) error {
	fmt.Println("listening...")
	return nil
}

func main() {
	srv := NewServer(8080)
	srv.Start(context.Background())
}
`)

	parser := NewParser()
	result, err := parser.Parse(context.Background(), "server.go", source, LangGo)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	fmt.Printf("\n=== Go 解析结果 ===\n")
	fmt.Printf("文件: %s | 语言: %s\n", result.FilePath, result.Language)
	fmt.Printf("实体数: %d | 关系数: %d\n\n", len(result.Entities), len(result.Relations))

	fmt.Println("── 实体 ──")
	for _, e := range result.Entities {
		fmt.Printf("  [%s] %s (L%d-%d)", e.Type, e.QualifiedName, e.LineStart, e.LineEnd)
		if e.ClassName != "" {
			fmt.Printf(" ∈ %s", e.ClassName)
		}
		if e.Params != "" {
			fmt.Printf(" params=%s", e.Params)
		}
		fmt.Println()
	}

	fmt.Println("\n── 关系 ──")
	for _, r := range result.Relations {
		fmt.Printf("  %s -[%s]-> %s\n", r.Source, r.Type, r.Target)
	}

	// 验证
	found := map[string]bool{}
	for _, e := range result.Entities {
		found[e.QualifiedName] = true
	}
	for _, expected := range []string{"Server", "Handler", "NewServer", "Server.Start", "Server.listen", "main"} {
		if !found[expected] {
			t.Errorf("Missing entity: %s", expected)
		}
	}

	// 检查 import 关系
	hasImport := false
	for _, r := range result.Relations {
		if r.Type == RelImports && r.Target == "context" {
			hasImport = true
		}
	}
	if !hasImport {
		t.Error("Missing: server.go -[IMPORTS]-> context")
	}
}

func TestParseTypeScript(t *testing.T) {
	source := []byte(`
import { Agent } from "./base";
import { Tool } from "@langchain/core/tools";

class ToolManager {
    private tools: Map<string, Tool>;

    constructor() {
        this.tools = new Map();
    }

    register(name: string, tool: Tool): void {
        this.tools.set(name, tool);
    }

    get(name: string): Tool | undefined {
        return this.tools.get(name);
    }
}

class AgentRunner extends Agent {
    private manager: ToolManager;

    constructor(model: string) {
        super(model);
        this.manager = new ToolManager();
    }

    async run(prompt: string): Promise<string> {
        const result = await this.process(prompt);
        return result;
    }
}

function createRunner(config: any): AgentRunner {
    return new AgentRunner(config.model);
}

export { AgentRunner, ToolManager, createRunner };
`)

	parser := NewParser()
	result, err := parser.Parse(context.Background(), "agent.ts", source, LangTypeScript)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	fmt.Printf("\n=== TypeScript 解析结果 ===\n")
	fmt.Printf("文件: %s | 语言: %s\n", result.FilePath, result.Language)
	fmt.Printf("实体数: %d | 关系数: %d\n\n", len(result.Entities), len(result.Relations))

	fmt.Println("── 实体 ──")
	for _, e := range result.Entities {
		fmt.Printf("  [%s] %s (L%d-%d)", e.Type, e.QualifiedName, e.LineStart, e.LineEnd)
		if e.ClassName != "" {
			fmt.Printf(" ∈ %s", e.ClassName)
		}
		fmt.Println()
	}

	fmt.Println("\n── 关系 ──")
	for _, r := range result.Relations {
		fmt.Printf("  %s -[%s]-> %s\n", r.Source, r.Type, r.Target)
	}

	// 验证
	if len(result.Entities) < 3 {
		t.Errorf("Expected at least 3 entities, got %d", len(result.Entities))
	}

	// 检查 import
	hasImport := false
	for _, r := range result.Relations {
		if r.Type == RelImports && r.Target == "./base" {
			hasImport = true
		}
	}
	if !hasImport {
		t.Error("Missing: agent.ts -[IMPORTS]-> ./base")
	}
}
