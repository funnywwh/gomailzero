import axios from 'axios'

const apiClient = axios.create({
  baseURL: '/api',
  headers: {
    'Content-Type': 'application/json'
  }
})

// 请求拦截器：添加 token
apiClient.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// 响应拦截器：处理错误
apiClient.interceptors.response.use(
  (response) => response.data,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('token')
      window.location.href = '/login'
    }
    return Promise.reject(error)
  }
)

export const api = {
  // 登录
  login: (data: { email: string; password: string; totp_code?: string }) =>
    apiClient.post('/login', data),

  // 获取邮件列表
  getMails: (folder?: string, limit?: number, offset?: number) =>
    apiClient.get('/mails', { params: { folder, limit, offset } }),

  // 获取邮件详情
  getMail: (id: string) => apiClient.get(`/mails/${id}`),

  // 发送邮件
  sendMail: (data: {
    to: string[]
    subject: string
    body: string
    cc?: string[]
    bcc?: string[]
  }) => apiClient.post('/mails', data),

  // 删除邮件
  deleteMail: (id: string) => apiClient.delete(`/mails/${id}`),

  // 更新邮件标志
  updateMailFlags: (id: string, flags: string[]) =>
    apiClient.put(`/mails/${id}/flags`, { flags }),

  // 搜索邮件
  searchMails: (query: string, folder?: string, limit?: number, offset?: number) =>
    apiClient.get('/mails/search', { params: { q: query, folder, limit, offset } }),

  // 列出文件夹
  listFolders: () => apiClient.get('/folders'),

  // 保存草稿
  saveDraft: (data: {
    id?: string
    to: string[]
    subject: string
    body: string
    cc?: string[]
    bcc?: string[]
  }) => apiClient.post('/mails/drafts', data)
}

