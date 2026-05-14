package main

import (
	"testing"
)

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// 1. Pure JSON string → returns as-is
		{
			name:     "pure JSON object",
			input:    `{"key":"value"}`,
			expected: `{"key":"value"}`,
		},
		{
			name:     "pure JSON with whitespace",
			input:    "  \n  {\"key\":\"value\"}  \n  ",
			expected: `{"key":"value"}`,
		},

		// 2. Markdown code block with json tag
		{
			name:     "markdown json code block",
			input:    "```json\n{\"key\":\"value\"}\n```",
			expected: `{"key":"value"}`,
		},
		{
			name:     "markdown json code block with extra whitespace",
			input:    "```json  \n  {\"key\":\"value\"}  \n  ```",
			expected: `{"key":"value"}`,
		},
		{
			name:     "markdown json code block with surrounding text",
			input:    "Here is the result:\n\n```json\n{\"result\":\"ok\"}\n```\n\nDone.",
			expected: `{"result":"ok"}`,
		},

		// 3. Markdown code block without json tag
		{
			name:     "markdown plain code block",
			input:    "```\n{\"key\":\"value\"}\n```",
			expected: `{"key":"value"}`,
		},
		{
			name:     "markdown plain code block with surrounding text",
			input:    "Result:\n```\n{\"status\":200}\n```\nEnd",
			expected: `{"status":200}`,
		},

		// 4. Mixed text: Chinese analysis + JSON at end
		{
			name:     "Chinese analysis then JSON",
			input:    "根据分析，这个任务需要较长时间。{\"task_ancient_minutes\":120,\"reason\":\"复杂任务\"}",
			expected: `{"task_ancient_minutes":120,"reason":"复杂任务"}`,
		},
		{
			name:     "Chinese analysis with newlines then JSON",
			input:    "分析结果如下：\n\n这个任务涉及多个模块。\n\n{\"task_ancient_minutes\":45,\"reason\":\"简单修改\"}",
			expected: `{"task_ancient_minutes":45,"reason":"简单修改"}`,
		},

		// 5. Multiple JSON objects in text → extracts the last complete one
		{
			name:     "multiple JSON objects",
			input:    `{"first":1} some text {"second":2}`,
			expected: `{"second":2}`,
		},
		{
			name:     "multiple nested JSON objects",
			input:    `{"a":{"b":1}} middle {"c":{"d":2}}`,
			expected: `{"c":{"d":2}}`,
		},

		// 6. Invalid JSON with braces but not valid → returns original text
		{
			name:     "invalid JSON with braces",
			input:    `{"key": value without quotes}`,
			expected: `{"key": value without quotes}`,
		},
		{
			name:     "unmatched braces",
			input:    `{"key": "value"`,
			expected: `{"key": "value"`,
		},

		// 7. Empty string → returns empty
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "whitespace only",
			input:    "   \n\t  ",
			expected: "",
		},

		// 8. JSON with nested objects
		{
			name:     "nested objects",
			input:    `{"outer":{"inner":{"deep":"value"}}}`,
			expected: `{"outer":{"inner":{"deep":"value"}}}`,
		},
		{
			name:     "deeply nested mixed arrays and objects",
			input:    `{"a":{"b":[1,{"c":{"d":2}}]}}`,
			expected: `{"a":{"b":[1,{"c":{"d":2}}]}}`,
		},

		// 9. JSON with strings containing braces/brackets
		{
			name:     "string containing closing brace",
			input:    `{"key": "value with } brace"}`,
			expected: `{"key": "value with } brace"}`,
		},
		{
			name:     "string containing opening brace",
			input:    `{"key": "value with { brace"}`,
			expected: `{"key": "value with { brace"}`,
		},
		{
			name:     "string containing both braces",
			input:    `{"key": "{nested}"}`,
			expected: `{"key": "{nested}"}`,
		},
		{
			name:     "string containing brackets",
			input:    `{"key": "value with [bracket]"}`,
			expected: `{"key": "value with [bracket]"}`,
		},

		// 10. Text with no JSON at all → returns original text
		{
			name:     "plain text no JSON",
			input:    "This is just plain text without any JSON.",
			expected: "This is just plain text without any JSON.",
		},
		{
			name:     "text with braces but no JSON",
			input:    "Hello {world} and [test]",
			expected: "Hello {world} and [test]",
		},

		// 11. JSON array at end
		{
			name:     "pure JSON array",
			input:    `[1, 2, 3]`,
			expected: `[1, 2, 3]`,
		},
		{
			name:     "pure JSON array of objects",
			input:    `[{"a":1},{"b":2}]`,
			expected: `[{"a":1},{"b":2}]`,
		},

		// 12. Escaped quotes inside strings
		{
			name:     "escaped quotes in string",
			input:    `{"key": "value with \"quoted\" text"}`,
			expected: `{"key": "value with \"quoted\" text"}`,
		},
		{
			name:     "escaped quotes in markdown block",
			input:    "```json\n{\"key\": \"value with \\\"quoted\\\" text\"}\n```",
			expected: `{"key": "value with \"quoted\" text"}`,
		},

		// Additional edge cases
		{
			name:     "JSON with numbers and booleans",
			input:    `{"count": 42, "active": true, "rate": 3.14}`,
			expected: `{"count": 42, "active": true, "rate": 3.14}`,
		},
		{
			name:     "JSON with null",
			input:    `{"value": null}`,
			expected: `{"value": null}`,
		},
		{
			name:     "single character string",
			input:    "a",
			expected: "a",
		},
		{
			name:     "only opening brace",
			input:    "{",
			expected: "{",
		},
		{
			name:     "only closing brace",
			input:    "}",
			expected: "}",
		},
		{
			name:     "markdown block with invalid JSON",
			input:    "```json\n{invalid}\n```",
			expected: "```json\n{invalid}\n```",
		},
		{
			name:     "multiple markdown blocks uses first valid",
			input:    "```\n{\"first\":1}\n```\n```\n{\"second\":2}\n```",
			expected: `{"first":1}`,
		},
		{
			name:     "JSON at start then plain text extracts first JSON",
			input:    `{"key":"value"} followed by text`,
			expected: `{"key":"value"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractJSON(tt.input)
			if got != tt.expected {
				t.Errorf("extractJSON(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
