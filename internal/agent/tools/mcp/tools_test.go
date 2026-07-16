package mcp

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestNormalizeCallToolResult(t *testing.T) {
	t.Parallel()

	rawImage := []byte{0x89, 0x50, 0x4e, 0x47}
	rawAudio := []byte{0xff, 0xf1, 0x50, 0x80}

	tests := []struct {
		name        string
		result      *mcp.CallToolResult
		want        ToolResult
		wantErr     string
		wantContent []string
	}{
		{
			name: "one text part",
			result: &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "hello"}},
			},
			want: ToolResult{Type: "text", Content: "hello"},
		},
		{
			name: "multiple text parts preserve order",
			result: &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: "first"},
					&mcp.TextContent{Text: "second"},
				},
			},
			want: ToolResult{Type: "text", Content: "first\nsecond"},
		},
		{
			name: "structured only becomes compact JSON",
			result: &mcp.CallToolResult{
				StructuredContent: map[string]any{
					"bases": []any{map[string]any{"id": "app123"}},
				},
			},
			want: ToolResult{
				Type:    "text",
				Content: `{"bases":[{"id":"app123"}]}`,
			},
		},
		{
			name: "ordinary content wins over structured content",
			result: &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: "authoritative"},
				},
				StructuredContent: map[string]any{"ignored": true},
			},
			want: ToolResult{Type: "text", Content: "authoritative"},
		},
		{
			name: "empty text part remains authoritative",
			result: &mcp.CallToolResult{
				Content:           []mcp.Content{&mcp.TextContent{Text: ""}},
				StructuredContent: map[string]any{"ignored": true},
			},
			want: ToolResult{Type: "text", Content: ""},
		},
		{
			name:   "empty result remains empty text",
			result: &mcp.CallToolResult{},
			want:   ToolResult{Type: "text", Content: ""},
		},
		{
			name: "text plus image preserves image response",
			result: &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: "caption"},
					&mcp.ImageContent{Data: rawImage, MIMEType: "image/png"},
				},
			},
			want: ToolResult{
				Type:      "image",
				Content:   "caption",
				Data:      rawImage,
				MediaType: "image/png",
			},
		},
		{
			name: "text plus audio preserves media response",
			result: &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: "caption"},
					&mcp.AudioContent{Data: rawAudio, MIMEType: "audio/aac"},
				},
			},
			want: ToolResult{
				Type:      "media",
				Content:   "caption",
				Data:      rawAudio,
				MediaType: "audio/aac",
			},
		},
		{
			name: "image continues to win over audio",
			result: &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.AudioContent{Data: rawAudio, MIMEType: "audio/aac"},
					&mcp.ImageContent{Data: rawImage, MIMEType: "image/png"},
				},
			},
			want: ToolResult{
				Type:      "image",
				Data:      rawImage,
				MediaType: "image/png",
			},
		},
		{
			name: "first image continues to win",
			result: &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.ImageContent{Data: rawImage, MIMEType: "image/png"},
					&mcp.ImageContent{Data: []byte{0xff, 0xd8}, MIMEType: "image/jpeg"},
				},
			},
			want: ToolResult{
				Type:      "image",
				Data:      rawImage,
				MediaType: "image/png",
			},
		},
		{
			name: "unknown content remains represented as text",
			result: &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.ResourceLink{
						URI:  "file:///tmp/report.txt",
						Name: "report",
					},
				},
			},
			want:        ToolResult{Type: "text"},
			wantContent: []string{"file:///tmp/report.txt", "report"},
		},
		{
			name: "text error preserves error bit",
			result: &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "denied"}},
				IsError: true,
			},
			want: ToolResult{Type: "text", Content: "denied", IsError: true},
		},
		{
			name: "structured error preserves error bit",
			result: &mcp.CallToolResult{
				StructuredContent: map[string]any{"error": "denied"},
				IsError:           true,
			},
			want: ToolResult{
				Type:    "text",
				Content: `{"error":"denied"}`,
				IsError: true,
			},
		},
		{
			name: "empty error preserves error bit",
			result: &mcp.CallToolResult{
				IsError: true,
			},
			want: ToolResult{Type: "text", Content: "", IsError: true},
		},
		{
			name: "unmarshalable structured content returns error",
			result: &mcp.CallToolResult{
				StructuredContent: map[string]any{"bad": func() {}},
			},
			wantErr: "marshal MCP structured content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := normalizeCallToolResult(tt.result)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			if len(tt.wantContent) == 0 {
				require.Equal(t, tt.want, got)
				return
			}
			require.Equal(t, tt.want.Type, got.Type)
			require.Equal(t, tt.want.IsError, got.IsError)
			for _, part := range tt.wantContent {
				require.Contains(t, got.Content, part)
			}
		})
	}
}

func TestNormalizeCallToolResultStructuredJSONIsValid(t *testing.T) {
	t.Parallel()

	result, err := normalizeCallToolResult(&mcp.CallToolResult{
		StructuredContent: map[string]any{
			"items": []any{1.0, "two", true},
		},
	})
	require.NoError(t, err)
	require.True(t, json.Valid([]byte(result.Content)))
	require.False(t, strings.Contains(result.Content, "\n"))
}

func TestEnsureRawBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    []byte
		wantData []byte
	}{
		{
			name:     "already base64 encoded",
			input:    []byte("SGVsbG8gV29ybGQh"), // "Hello World!" in base64
			wantData: []byte("Hello World!"),
		},
		{
			name:     "raw binary data (PNG header)",
			input:    []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A},
			wantData: []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A},
		},
		{
			name:     "raw binary with high bytes",
			input:    []byte{0xFF, 0xD8, 0xFF, 0xE0}, // JPEG header
			wantData: []byte{0xFF, 0xD8, 0xFF, 0xE0},
		},
		{
			name:     "empty data",
			input:    []byte{},
			wantData: []byte{},
		},
		{
			name:     "base64 with padding",
			input:    []byte("YQ=="), // "a" in base64
			wantData: []byte("a"),
		},
		{
			name:     "base64 without padding",
			input:    []byte("YQ"),
			wantData: []byte("a"),
		},
		{
			name:     "base64 with whitespace",
			input:    []byte("U0dWc2JHOGdWMjl5YkdRaA==\n"),
			wantData: []byte("SGVsbG8gV29ybGQh"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ensureRawBytes(tt.input)
			require.Equal(t, tt.wantData, result)

			if len(result) > 0 && !bytes.Equal(result, tt.input) {
				reEncoded := base64.StdEncoding.EncodeToString(result)
				_, err := base64.StdEncoding.DecodeString(reEncoded)
				require.NoError(t, err, "re-encoded result should be valid base64")
			}
		})
	}
}

func TestFilterTools(t *testing.T) {
	t.Parallel()

	tools := []*Tool{
		{Name: "tool_a"},
		{Name: "tool_b"},
		{Name: "tool_c"},
	}

	t.Run("no filters returns all tools", func(t *testing.T) {
		t.Parallel()
		result := filterTools(config.MCPConfig{}, tools)
		require.Len(t, result, 3)
	})

	t.Run("disabled tools filters deny list", func(t *testing.T) {
		t.Parallel()
		result := filterTools(config.MCPConfig{DisabledTools: []string{"tool_a"}}, tools)
		require.Len(t, result, 2)
		require.Equal(t, "tool_b", result[0].Name)
		require.Equal(t, "tool_c", result[1].Name)
	})

	t.Run("enabled tools acts as allow list", func(t *testing.T) {
		t.Parallel()
		result := filterTools(config.MCPConfig{EnabledTools: []string{"tool_b"}}, tools)
		require.Len(t, result, 1)
		require.Equal(t, "tool_b", result[0].Name)
	})

	t.Run("enabled and disabled both apply", func(t *testing.T) {
		t.Parallel()
		result := filterTools(config.MCPConfig{
			EnabledTools:  []string{"tool_a", "tool_b"},
			DisabledTools: []string{"tool_b"},
		}, tools)
		require.Len(t, result, 1)
		require.Equal(t, "tool_a", result[0].Name)
	})

	t.Run("enabled with non-existent tool returns empty", func(t *testing.T) {
		t.Parallel()
		result := filterTools(config.MCPConfig{EnabledTools: []string{"non_existent"}}, tools)
		require.Len(t, result, 0)
	})
}
