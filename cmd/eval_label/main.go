package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type EvalSample struct {
	ID             string   `json:"id"`
	Question       string   `json:"question"`
	Category       string   `json:"category,omitempty"`
	GoldDocs       []string `json:"gold_docs,omitempty"`
	AnswerKeywords []string `json:"answer_keywords,omitempty"`
	ExpectedAnswer string   `json:"expected_answer,omitempty"`
	JudgeRule      string   `json:"judge_rule,omitempty"`
	CandidateDocs  []string `json:"candidate_docs,omitempty"`
	NeedsReview    bool     `json:"needs_review,omitempty"`
}

type chatRequest struct {
	Message string `json:"message"`
	Mode    string `json:"mode"`
}

type reference struct {
	ID string `json:"id"`
}

type chatResponse struct {
	References []reference `json:"references"`
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	AccessToken string `json:"access_token"`
}

func main() {
	input := flag.String("input", "data/eval_set.jsonl", "输入评测集（jsonl）")
	output := flag.String("output", "data/eval_set_labeled.jsonl", "输出带 candidate_docs 的 jsonl")
	baseURL := flag.String("base-url", "http://localhost:8080", "服务地址")
	mode := flag.String("mode", "pipeline", "聊天模式")
	token := flag.String("token", "", "Bearer token（可选）")
	username := flag.String("username", "", "登录用户名（可选）")
	password := flag.String("password", "", "登录密码（可选）")
	timeoutSec := flag.Int("timeout", 60, "请求超时（秒）")
	overwrite := flag.Bool("overwrite", false, "是否覆盖已有 candidate_docs")
	flag.Parse()

	samples, err := loadSamples(*input)
	if err != nil {
		fmt.Printf("[label] 读取输入失败: %v\n", err)
		os.Exit(1)
	}

	client := &http.Client{Timeout: time.Duration(*timeoutSec) * time.Second}
	bearer := strings.TrimSpace(*token)
	if bearer == "" && *username != "" && *password != "" {
		bearer, err = login(client, strings.TrimRight(*baseURL, "/"), *username, *password)
		if err != nil {
			fmt.Printf("[label] 登录失败: %v\n", err)
			os.Exit(1)
		}
	}

	for idx := range samples {
		s := &samples[idx]
		if strings.TrimSpace(s.Question) == "" {
			continue
		}
		if len(s.CandidateDocs) > 0 && !*overwrite {
			continue
		}
		cand, err := fetchCandidates(client, strings.TrimRight(*baseURL, "/"), bearer, *mode, s.Question)
		if err != nil {
			fmt.Printf("[label] %s error: %v\n", s.ID, err)
			s.NeedsReview = true
			continue
		}
		s.CandidateDocs = cand
		s.NeedsReview = true
		fmt.Printf("[label] %s candidate_docs=%d\n", s.ID, len(cand))
	}

	if err := writeSamples(*output, samples); err != nil {
		fmt.Printf("[label] 写入输出失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("[label] done: %s\n", *output)
}

func loadSamples(path string) ([]EvalSample, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	out := make([]EvalSample, 0)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var s EvalSample
		if err := json.Unmarshal([]byte(line), &s); err != nil {
			continue
		}
		if s.ID == "" {
			s.ID = fmt.Sprintf("sample_%d", len(out)+1)
		}
		out = append(out, s)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func writeSamples(path string, samples []EvalSample) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	for _, s := range samples {
		b, err := json.Marshal(s)
		if err != nil {
			continue
		}
		if _, err := w.Write(append(b, '\n')); err != nil {
			return err
		}
	}
	return w.Flush()
}

func login(client *http.Client, baseURL, username, password string) (string, error) {
	body, _ := json.Marshal(loginRequest{Username: username, Password: password})
	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/auth/login", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var lr loginResponse
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		return "", err
	}
	if lr.AccessToken == "" {
		return "", fmt.Errorf("登录响应缺少 access_token")
	}
	return lr.AccessToken, nil
}

func fetchCandidates(client *http.Client, baseURL, bearer, mode, question string) ([]string, error) {
	body, _ := json.Marshal(chatRequest{Message: question, Mode: mode})
	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/chat", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var cr chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return nil, err
	}

	seen := map[string]struct{}{}
	out := make([]string, 0, len(cr.References))
	for _, r := range cr.References {
		id := strings.TrimSpace(r.ID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
}
