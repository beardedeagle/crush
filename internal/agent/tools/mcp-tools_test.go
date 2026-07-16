package tools

import (
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/stretchr/testify/require"
)

func TestNewMCPToolResponse(t *testing.T) {
	t.Parallel()

	imageData := []byte{0x89, 0x50, 0x4e, 0x47}
	audioData := []byte{0xff, 0xf1, 0x50, 0x80}

	tests := []struct {
		name           string
		result         mcp.ToolResult
		supportsImages bool
		modelName      string
		want           fantasy.ToolResponse
	}{
		{
			name:   "successful text",
			result: mcp.ToolResult{Type: "text", Content: "ok"},
			want:   fantasy.NewTextResponse("ok"),
		},
		{
			name: "tool error becomes Fantasy error",
			result: mcp.ToolResult{
				Type:    "text",
				Content: "denied",
				IsError: true,
			},
			want: fantasy.NewTextErrorResponse("denied"),
		},
		{
			name: "successful image remains image",
			result: mcp.ToolResult{
				Type:      "image",
				Content:   "caption",
				Data:      imageData,
				MediaType: "image/png",
			},
			supportsImages: true,
			want: fantasy.ToolResponse{
				Type:      "image",
				Content:   "caption",
				Data:      imageData,
				MediaType: "image/png",
			},
		},
		{
			name: "successful audio remains media",
			result: mcp.ToolResult{
				Type:      "media",
				Content:   "caption",
				Data:      audioData,
				MediaType: "audio/aac",
			},
			supportsImages: true,
			want: fantasy.ToolResponse{
				Type:      "media",
				Content:   "caption",
				Data:      audioData,
				MediaType: "audio/aac",
			},
		},
		{
			name: "unsupported image remains model capability error",
			result: mcp.ToolResult{
				Type:      "image",
				Data:      imageData,
				MediaType: "image/png",
			},
			modelName: "text-only",
			want: fantasy.NewTextErrorResponse(
				"This model (text-only) does not support image data.",
			),
		},
		{
			name: "MCP error takes precedence over media capability",
			result: mcp.ToolResult{
				Type:      "image",
				Content:   "server denied request",
				Data:      imageData,
				MediaType: "image/png",
				IsError:   true,
			},
			modelName: "text-only",
			want:      fantasy.NewTextErrorResponse("server denied request"),
		},
		{
			name: "empty MCP error retains error flag",
			result: mcp.ToolResult{
				Type:    "text",
				IsError: true,
			},
			want: fantasy.NewTextErrorResponse(""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := newMCPToolResponse(
				tt.result,
				tt.supportsImages,
				tt.modelName,
			)
			require.Equal(t, tt.want, got)
		})
	}
}
