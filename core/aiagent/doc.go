// Package aiagent 提供 AI 大模型客户端（兼容 OpenAI API 格式）。
//
// 唯一职责：
//   - NewGLMClient：创建 AI 客户端
//   - NewClientFromEnv：从环境变量创建客户端（推荐使用）
//   - RunAnalysis：执行 AI 分析
//
// 禁止事项：
//   - 禁止在 cmd_*.go 中内联 os.Getenv("OPENAI_API_KEY") + NewGLMClient() 初始化代码
//   - 所有 AI 客户端初始化必须通过 NewClientFromEnv() 工厂函数
//
// 使用示例：
//
//	client, err := aiagent.NewClientFromEnv("glm-4")
//	if err != nil { return err }
package aiagent
