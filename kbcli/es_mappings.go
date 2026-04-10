package main

// RequestIndexMapping request 层索引 mapping
const RequestIndexMapping = `{
  "mappings": {
    "properties": {
      "@timestamp":               { "type": "date" },
      "api_request_time":         { "type": "date" },
      "api_end_time":             { "type": "date" },
      "task_id":                  { "type": "keyword" },
      "request_id":               { "type": "keyword" },
      "client_id":                { "type": "keyword" },
      "user_id":                  { "type": "keyword" },
      "user_name":                { "type": "keyword" },
      "repo_id":                  { "type": "keyword" },
      "project_id":               { "type": "keyword" },
      "project_path":             { "type": "keyword" },
      "caller":                   { "type": "keyword" },
      "sender":                   { "type": "keyword" },
      "model":                    { "type": "keyword" },
      "client_ide":               { "type": "keyword" },
      "client_version":           { "type": "keyword" },
      "client_os":                { "type": "keyword" },
      "prompt_mode":              { "type": "keyword" },
      "mode":                     { "type": "keyword" },
      "user_in_chars":            { "type": "long" },
      "assistant_out_code_lines": { "type": "long" },
      "system_tokens":            { "type": "long" },
      "user_tokens":              { "type": "long" },
      "api_in_tokens":            { "type": "long" },
      "api_out_tokens":           { "type": "long" },
      "api_process_time":         { "type": "long" },
      "api_ttft":                 { "type": "long" },
      "api_cost":                 { "type": "float" },
      "org1":                     { "type": "keyword" },
      "org2":                     { "type": "keyword" },
      "org3":                     { "type": "keyword" },
      "org4":                     { "type": "keyword" },
      "source_path":              { "type": "keyword" }
    }
  }
}`

// TaskIndexMapping task 层索引 mapping
const TaskIndexMapping = `{
  "mappings": {
    "properties": {
      "@timestamp":               { "type": "date" },
      "caller":                   { "type": "keyword" },
      "task_id":                  { "type": "keyword" },
      "client_id":                { "type": "keyword" },
      "user_id":                  { "type": "keyword" },
      "user_name":                { "type": "keyword" },
      "repo_id":                  { "type": "keyword" },
      "project_path":             { "type": "keyword" },
      "project_id":               { "type": "keyword" },
      "client_ide":               { "type": "keyword" },
      "client_version":           { "type": "keyword" },
      "client_os":                { "type": "keyword" },
      "prompt_mode":              { "type": "keyword" },
      "mode":                     { "type": "keyword" },
      "org1":                     { "type": "keyword" },
      "org2":                     { "type": "keyword" },
      "org3":                     { "type": "keyword" },
      "org4":                     { "type": "keyword" },
      "user_in_chars":            { "type": "long" },
      "assistant_out_code_lines": { "type": "long" },
      "system_tokens":            { "type": "long" },
      "user_tokens":              { "type": "long" },
      "api_request_time":         { "type": "date" },
      "api_end_time":             { "type": "date" },
      "api_process_time":         { "type": "long" },
      "api_ttft":                 { "type": "long" },
      "api_in_tokens":            { "type": "long" },
      "api_out_tokens":           { "type": "long" },
      "api_cost":                 { "type": "float" },
      "api_count":                { "type": "long" },
      "source_file":              { "type": "keyword" },
      "task_ancient_minutes":      { "type": "float" },
      "task_ancient_minutes_reason": { "type": "text" }
    }
  }
}`
