package handler

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"

	"eino_agent/internal/usage"
)

// Heartbeat answers a workspace controller's poll. It must stay cheap and must
// not call the LLM or the database: the controller treats a slow or failing
// reply as an unresponsive process and restarts it.
//
// Fields follow agent-workspace's heartbeat contract:
//   - busy: a chat turn is running, so the workspace is not idle even if no new
//     request has arrived for a while;
//   - version: AGENT_VERSION when set (the image tag), else the build version;
//   - usage: cumulative LLM tokens, non-decreasing across restarts.
func (h *Handler) Heartbeat(c *gin.Context) {
	version := os.Getenv("AGENT_VERSION")
	if version == "" {
		version = Version
	}
	c.JSON(http.StatusOK, gin.H{
		"busy":    usage.Default.InFlight() > 0,
		"version": version,
		"usage":   usage.Default.Totals(),
	})
}

// TrackInFlight counts a request as in-flight work until its handler returns,
// which for a streamed reply is when the stream ends.
func (h *Handler) TrackInFlight() gin.HandlerFunc {
	return func(c *gin.Context) {
		end := usage.Default.Begin()
		defer end()
		c.Next()
	}
}
