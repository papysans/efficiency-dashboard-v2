import axios, { type AxiosInstance, type AxiosRequestConfig } from 'axios'

// 对齐 Vue frontend/src/api/index.js：baseURL=/api，timeout 30s，错误体约定 { error: string }。
// dev 时 vite proxy /api → localhost:9990；生产时 nginx 反代。见 research/api-contract.md §1。
export const http: AxiosInstance = axios.create({
  baseURL: '/api',
  timeout: 30000,
})

http.interceptors.response.use(
  (response) => response,
  (error) => {
    const msg = error?.response?.data?.error || '请求失败，请稍后重试'
    // 统一抛出带可读 message 的 Error，UI 层（React Query error / toast）消费。
    return Promise.reject(new Error(msg))
  },
)

// 统一解包：业务数据在 response.data，调用方直接拿数据。
export async function apiGet<T>(url: string, params?: Record<string, unknown>): Promise<T> {
  const res = await http.get<T>(url, { params })
  return res.data
}

export async function apiPost<T>(url: string, body?: unknown, config?: AxiosRequestConfig): Promise<T> {
  const res = await http.post<T>(url, body, config)
  return res.data
}

export async function apiPut<T>(url: string, body?: unknown): Promise<T> {
  const res = await http.put<T>(url, body)
  return res.data
}

export async function apiDelete<T>(url: string, config?: AxiosRequestConfig): Promise<T> {
  const res = await http.delete<T>(url, config)
  return res.data
}
