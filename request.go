package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Request struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
	Tools    []any         `json:"tools"`
}

type ChatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCallId string     `json:"tool_call_id"`
	ToolCalls  []ToolCall `json:"tool_calls"`
}

type ToolCall struct {
	Id       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

func writeTagNl(sb *strings.Builder, tag, s string) {
	s = strings.TrimSpace(s)
	sb.WriteString(fmt.Sprintf("<%s>\n", tag))
	for _, line := range strings.Split(s, "\n") {
		sb.WriteString(fmt.Sprintf("  %s\n", line))
	}
	sb.WriteString(fmt.Sprintf("</%s>\n", tag))
}

func (req *Request) toolUsePart(sb *strings.Builder) {
	if len(req.Tools) > 0 {
		b, _ := json.MarshalIndent(req.Tools, "", "  ")
		content := fmt.Sprintf(`USE TOOL
--------
Specify what tool to use and the required arguments in <use></use> block.
- Place tool name in "tool" attribute.
- Place tool arguments between <use> and </use>
- The tool arguments MUST be a valid JSON object that can be validated against the tool's input JSON schema.
ALWAYS check the existing facts before calling the tool, DO NOT call tools repeatedly.
Once you respond with </use>, you STOP.

<examples>
	<good_example>
		<use tool="Edit">
		{
			"file_path": "/path/to/main.py",
			"old_string": "class Snippet:\n    def __init__(self, file_path, line_no, lines):",
			"new_string": "class Snippet:\n    def __init__(self, file_path, line_no, lines, context_range=4):"
		}
		</use>
	</good_example>

	<bad_example>
		<use>
		{
			"tool": "Edit",
			"file_path": "/path/to/main.py",
			"old_string": "class Snippet:\n    def __init__(self, file_path, line_no, lines):",
			"new_string": "class Snippet:\n    def __init__(self, file_path, line_no, lines, context_range=4):"
		}
		</use>
		<reasoning>
			The tool name should be placed in "tool" attribute!
		</reasoning>
	</bad_example>

	<bad_example>
		<use tool="Read">
			<file_path>/path/to/main.go</file_path>
			<offset>116</offset>
			<limit>110</limit>
		</use>
		<reasoning>
			The tool arguments MUST be a valid JSON object.
		</reasoning>
	</bad_example>
</examples>

AVAILABLE TOOLS
---------------
%s`, string(b))
		writeTagNl(sb, "tools", content)
	}
}

func (req *Request) Compose() string {
	if len(req.Messages) == 0 {
		return ""
	}
	var sb strings.Builder
	messages := req.Messages
	firstMsg := messages[0]
	if firstMsg.Role == "system" {
		writeTagNl(&sb, firstMsg.Role, firstMsg.Content)
		req.toolUsePart(&sb)
		messages = messages[1:]
	}
	for _, m := range messages {
		if m.ToolCallId != "" {
			writeTagNl(&sb, m.Role, fmt.Sprintf("tool_call_id: %s, content: %s", m.ToolCallId, m.Content))
		} else if m.ToolCalls != nil {
			b, _ := json.MarshalIndent(m.ToolCalls, "", "  ")
			writeTagNl(&sb, m.Role, string(b))
		} else {
			writeTagNl(&sb, m.Role, m.Content)
		}
	}
	return sb.String()
}
