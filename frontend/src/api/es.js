import request from './index'

export function getRequests(params) {
  return request({ url: '/requests', method: 'get', params })
}

export function getAggregate(params) {
  return request({ url: '/aggregate', method: 'get', params })
}

export function getAggregateKeys(params) {
  return request({ url: '/aggregate/keys', method: 'get', params })
}

export function getEfficiency(params) {
  return request({ url: '/analysis/efficiency', method: 'get', params })
}

export function calculateEfficiency(data) {
  return request({ url: '/analysis/efficiency/calculate', method: 'post', data })
}

export function correctEfficiency(data) {
  return request({ url: '/analysis/efficiency/correct', method: 'put', data })
}

export function getEfficiencyHistory(params) {
  return request({ url: '/analysis/efficiency/history', method: 'get', params })
}

export function getEfficiencyFile(params) {
  return request({ url: '/analysis/efficiency/file', method: 'get', params })
}

export function updateManualDays(data) {
  return request({ url: '/analysis/efficiency/manual-days', method: 'put', data })
}

export function getGitAnalysis(params) {
  return request({ url: '/analysis/git', method: 'get', params })
}

export function getTaskCommitMappings(params) {
  return request({ url: '/analysis/task-commits', method: 'get', params })
}

export function getCodeAttribution(params) {
  return request({ url: '/analysis/code-attribution', method: 'get', params })
}

export function getCodeSourceStats(params) {
  return request({ url: '/analysis/code-source', method: 'get', params })
}

export function createVirtualGroup(data) {
  return request({ url: '/virtual-groups', method: 'post', data })
}

export function getVirtualGroupAggregate(id, params) {
  return request({ url: `/virtual-groups/${id}/aggregate`, method: 'get', params })
}

export function addFavorite(data) {
  return request({ url: '/favorites', method: 'post', data })
}

export function getFavorites(params) {
  return request({ url: '/favorites', method: 'get', params })
}

export function removeFavorite(id) {
  return request({ url: `/favorites/${id}`, method: 'delete' })
}

export function getTasksV2(params) {
  return request({ url: '/v2/tasks', method: 'get', params })
}

export function getTaskDetailV2(taskId) {
  return request({ url: `/v2/tasks/${taskId}`, method: 'get' })
}

export function getCommitsV2(params) {
  return request({ url: '/v2/commits', method: 'get', params })
}

export function getCommitDetailV2(commitId) {
  return request({ url: `/v2/commits/${commitId}`, method: 'get' })
}

export function getUsersV2(params) {
  return request({ url: '/v2/users', method: 'get', params })
}

export function getUserDetailV2(userId, params) {
  return request({ url: `/v2/users/${userId}`, method: 'get', params })
}

export function getOrgV2(params) {
  return request({ url: '/v2/orgs', method: 'get', params })
}

export function getOrgDetailV2(params) {
  return request({ url: '/v2/orgs/detail', method: 'get', params })
}

export function getDashboardSummary(params) {
  return request({ url: '/v2/dashboard/summary', method: 'get', params })
}

export function getReposV2(params) {
  return request({ url: '/v2/repos', method: 'get', params })
}

export function getRepoDetailV2New(repoAddr, repoBranch, params) {
  return request({ url: '/v2/repos/detail', method: 'get', params: { repoAddr, repoBranch, ...params } })
}

export function getRepoBranches(repoAddr) {
  return request({ url: '/v2/repos/branches', method: 'get', params: { repoAddr } })
}

export function updateTaskManualV2(taskId, data) {
  return request({ url: `/v2/tasks/${taskId}/manual`, method: 'put', data })
}

export function updateCommitManualV2(commitId, data) {
  return request({ url: `/v2/commits/${commitId}/manual`, method: 'put', data })
}

export function getTaskFileV2(params) {
  return request({ url: '/v2/tasks/file', method: 'get', params })
}

export function estimateAncientMinutes(params) {
  return request({ url: '/v2/tasks/estimate-ancient', method: 'post', params, timeout: 600000 })
}

// === Project V2 API ===
export const createProject = (data) => request({ url: '/v2/projects', method: 'post', data })
export const getProjects = () => request({ url: '/v2/projects', method: 'get' })
export const getProjectDetail = (projectId) => request({ url: `/v2/projects/${projectId}`, method: 'get' })
export const updateProject = (projectId, data) => request({ url: `/v2/projects/${projectId}`, method: 'put', data })
export const deleteProject = (projectId) => request({ url: `/v2/projects/${projectId}`, method: 'delete' })
export const updateProjectManual = (projectId, data) => request({ url: `/v2/projects/${projectId}/manual`, method: 'put', data })
export const addTasksToProject = (projectId, data) => request({ url: `/v2/projects/${projectId}/tasks`, method: 'post', data })
export const addRepoToProject = (projectId, data) => request({ url: `/v2/projects/${projectId}/repos`, method: 'post', data })
export const removeRepoFromProject = (projectId, index) => request({ url: `/v2/projects/${projectId}/repos/${index}`, method: 'delete' })
export const checkProjectConflicts = (data) => request({ url: '/v2/projects/check-conflicts', method: 'post', data })

// === User Productivity API ===
export const rebuildUsersV2 = (params) => request({ url: '/v2/users/rebuild', method: 'post', params })

// === User Groups API ===
export const createUserGroup = (data) => request({ url: '/v2/user-groups', method: 'post', data })
export const getUserGroups = () => request({ url: '/v2/user-groups', method: 'get' })
export const deleteUserGroup = (groupId) => request({ url: `/v2/user-groups/${groupId}`, method: 'delete' })
export const getUserGroupDetail = (groupId, params) => request({ url: `/v2/user-groups/${groupId}`, method: 'get', params })

export const removeTasksFromProject = (projectId, data) => request({ url: `/v2/projects/${projectId}/tasks`, method: 'delete', data })
export const updateTaskSilicaInProject = (projectId, data) => request({ url: `/v2/projects/${projectId}/tasks/silica`, method: 'put', data })
export const getGlobalConfig = () => request({ url: '/v2/config', method: 'get' })
export const getGroupDetail = (params) => request({ url: '/v2/group', method: 'get', params })
