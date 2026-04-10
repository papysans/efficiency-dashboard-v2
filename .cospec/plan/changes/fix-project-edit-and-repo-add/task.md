## 实施

- [x] 1.1 修复 Project 编辑时数据被全量覆盖清空的 Bug
     【目标对象】`frontend/src/views/ProjectDetailV2.vue` → `submitEdit()` 函数（第 543-562 行）
     【修改目的】`submitEdit()` 当前只提交 name/description 两个字段，后端 `updateProjectV2` 执行全量覆盖，导致 repos/task_ids/task_ids_silica 被清空；修复后这三个字段应保留原始值不变
     【修改方式】修改 `submitEdit()` 函数中传给 `updateProject()` 的 data 对象，追加三个字段
     【相关依赖】`project.value`（由 `loadData()` 从后端获取，包含 repos/task_ids/task_ids_silica 原始数据）
     【修改内容】
        - 在 `updateProject()` 调用的 data 对象中，追加 repos、task_ids、task_ids_silica 三个字段
        - 三个字段的值直接取自 `project.value` 对应属性，保持原始值不变，不做任何转换
        - 不修改 editForm 的结构，editForm 仍只维护 name/description 两个可编辑字段

- [x] 1.2 排查并确认 Repo 详情页"添加到 Project"下拉列表数据解析是否正确
     【目标对象】`frontend/src/views/RepoDetailV2.vue` → `openAddToProject()` 函数（第 342-359 行）
     【修改目的】确认 `getProjects()` 返回数据能被正确解析为 projectList，使下拉列表正常展示已有 Project；若存在解析错误则修复
     【修改方式】核查 `openAddToProject()` 中的解析逻辑，与后端 `listProjectsV2`（`backend/project_handler_v2.go`）的实际返回结构进行比对
     【相关依赖】
        - 后端 `backend/project_handler_v2.go` → `listProjectsV2()`：返回结构为 `{ "data": [...] }`（数组元素含 project_id、name 等字段）
        - 前端 `frontend/src/api/es.js` → `getProjects()`：GET `/v2/projects`
        - 模板中 `el-option` 绑定：`:key="p.project_id"` / `:label="p.name"` / `:value="p.project_id"`
     【修改内容】
        - 核查当前解析路径：`result.data || result` 取出数组，再判断 `Array.isArray(data)` 赋值给 `projectList.value`
        - 与后端返回结构 `{ "data": [...] }` 比对：`result.data` 即为数组，`Array.isArray` 判断为 true，解析路径正确
        - 核查模板绑定字段名与后端返回字段名是否一致（project_id、name）
        - 若以上核查均通过，则无需修改此处；若发现字段名不一致或解析路径错误，则按实际情况修正解析逻辑或模板绑定
