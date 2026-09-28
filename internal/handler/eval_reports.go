package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type EvalReportView struct {
	Name       string `json:"name"`
	Size       int64  `json:"size"`
	ModifiedAt string `json:"modified_at"`
}

func (h *Handler) ListEvalReports(c *gin.Context) {
	reportDir := filepath.Join("docs", "eval_reports")
	entries, err := os.ReadDir(reportDir)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusOK, gin.H{"reports": []EvalReportView{}})
			return
		}
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	reports := make([]EvalReportView, 0)
	for _, entry := range entries {
		if entry.IsDir() || !(strings.HasSuffix(strings.ToLower(entry.Name()), ".md") || strings.HasSuffix(strings.ToLower(entry.Name()), ".json")) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		reports = append(reports, EvalReportView{
			Name:       entry.Name(),
			Size:       info.Size(),
			ModifiedAt: info.ModTime().Format(time.RFC3339),
		})
	}

	sort.Slice(reports, func(i, j int) bool {
		return reports[i].ModifiedAt > reports[j].ModifiedAt
	})

	if len(reports) > 30 {
		reports = reports[:30]
	}

	c.JSON(http.StatusOK, gin.H{"reports": reports})
}

func (h *Handler) GetEvalReport(c *gin.Context) {
	name := filepath.Base(c.Param("name"))
	if name != c.Param("name") || !(strings.HasSuffix(strings.ToLower(name), ".md") || strings.HasSuffix(strings.ToLower(name), ".json")) {
		respondError(c, http.StatusBadRequest, "非法报告名称")
		return
	}
	path := filepath.Join("docs", "eval_reports", name)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		respondError(c, http.StatusNotFound, "报告不存在")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	if strings.HasSuffix(strings.ToLower(name), ".json") {
		var report any
		if err := json.Unmarshal(data, &report); err != nil {
			respondError(c, http.StatusInternalServerError, "报告 JSON 无效")
			return
		}
		c.JSON(http.StatusOK, gin.H{"name": name, "report": report})
		return
	}
	c.JSON(http.StatusOK, gin.H{"name": name, "content": string(data)})
}
