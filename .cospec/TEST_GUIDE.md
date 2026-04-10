# TEST_GUIDE.md

## 项目概述
Go (Gin) + PostgreSQL + Vue 3 + Element Plus 的 AI Coding 指标看板

## 可运行性验证

### 后端编译
```powershell
go build ./...
```
工作目录: `backend/`

### 前端构建
```powershell
npm run build
```
工作目录: `frontend/`

## 测试执行

### 后端集成测试
```powershell
go test -tags integration -v -count=1 ./...
```
工作目录: `backend/`

说明:
- 使用 `//go:build integration` 构建标签
- 直接连接本地 PostgreSQL 数据库 `costrict_stat`（host=localhost, port=5432, user=postgres, password=1）
- 测试数据通过 `defer DELETE` 自动清理
- 使用 `httptest` + `gin.TestMode` 进行 HTTP handler 测试

### 运行特定测试
```powershell
go test -tags integration -run TestProject -v -count=1
```

### 测试文件命名规范
- `*_integration_test.go` - 集成测试（需要 `-tags integration`）
- `*_test.go` - 单元测试（不需要额外 tag）

## 数据库信息
- 主数据库: `report`（host=localhost, port=5432, user=postgres, password=1）
- 统计数据库: `costrict_stat`（host=localhost, port=5432, user=postgres, password=1）

## 测试计划文件
测试计划保存在 `.cospec/test-plans/` 目录下
