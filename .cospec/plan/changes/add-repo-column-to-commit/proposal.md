# 变更：CommitViewV2 添加仓库列

## 原因
CommitViewV2 页面缺少仓库信息，用户无法从 Commit 列表直接跳转到仓库详情页查看该仓库的统计数据。

## 变更内容

### 前端
- **CommitViewV2.vue**：在列定义中新增「仓库」列，显示 `repo_addr/repo_branch` 格式，点击跳转到仓库详情页 `/repo/{repoAddr}/{repoBranch}`，并携带当前日期筛选参数（`startDate`、`endDate`）

### 后端
- 无需修改，`listCommitsV2` 已返回 `repo_addr` 和 `repo_branch` 字段

## 影响

- **受影响的规范**：Commit 列表
- **受影响的代码**：
    - `frontend/src/views/CommitViewV2.vue`：新增「仓库」列定义和跳转逻辑
