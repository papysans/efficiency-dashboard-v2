import axios, { type AxiosInstance, type AxiosRequestConfig } from 'axios'

// baseURL=/kanban/api：整站经外层 ingress 以 /kanban 前缀进入，接口必须带 /kanban 网关才路由到本服务。
// dev 时 vite proxy /kanban/api → localhost:9990（剥 /kanban）；生产时 portal nginx location /kanban/api/ 反代。
export const http: AxiosInstance = axios.create({
  baseURL: '/kanban/api',
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

// ============================================================
// chat-indicator-statistics 代理专用通道（/api/v2/chat/*）
// chat 侧响应不是看板的「裸数据」约定，而是 {success,code,data} 信封
// （个别端点是 {code:0,data}；NilSuccess 无 data），错误是 400 + {error:{code,message,type}}；
// 看板代理层自身错误（未配置 503 / 上游挂了 502）则是 {error:"<string>"}。
// 为不污染主拦截器，单独建 axios 实例 + 解包，调用方拿到的仍是业务 data。
// ============================================================

const chatHttp: AxiosInstance = axios.create({
  baseURL: '/kanban/api/v2/chat',
  // 实时查询直查源库可能慢（设计 §1.2 后端 Transport 60s），前端留 65s 余量。
  timeout: 65000,
})

chatHttp.interceptors.response.use(
  (response) => response,
  (error) => {
    const data = error?.response?.data
    // chat 服务错误体 {error:{message}} 优先；代理层错误体 {error:"string"} 兜底。
    const msg =
      data?.error?.message ||
      (typeof data?.error === 'string' ? data.error : '') ||
      '请求失败，请稍后重试'
    return Promise.reject(new Error(msg))
  },
)

/** chat 信封：{success,code,data} 或 {code:0,data}；NilSuccess 时 data 缺省（解包得 undefined）。 */
interface ChatEnvelope<T> {
  success?: boolean
  code?: string | number
  data?: T
}

export async function chatGet<T>(url: string, params?: Record<string, unknown>): Promise<T> {
  const res = await chatHttp.get<ChatEnvelope<T>>(url, { params })
  return res.data?.data as T
}

export async function chatPost<T>(url: string, body?: unknown, config?: AxiosRequestConfig): Promise<T> {
  const res = await chatHttp.post<ChatEnvelope<T>>(url, body, config)
  return res.data?.data as T
}

export async function chatPut<T>(url: string, body?: unknown): Promise<T> {
  const res = await chatHttp.put<ChatEnvelope<T>>(url, body)
  return res.data?.data as T
}

export async function chatDelete<T>(url: string, config?: AxiosRequestConfig): Promise<T> {
  const res = await chatHttp.delete<ChatEnvelope<T>>(url, config)
  return res.data?.data as T
}
