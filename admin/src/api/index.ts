import axios, { AxiosInstance } from 'axios'

const apiClient: AxiosInstance = axios.create({
  baseURL: '/api/v1',
  headers: {
    'Content-Type': 'application/json'
  }
})

// 请求拦截器：添加 JWT token
apiClient.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('admin_token')
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  (error) => {
    return Promise.reject(error)
  }
)

// 响应拦截器：处理错误和 token 过期
apiClient.interceptors.response.use(
  (response) => {
    return response.data
  },
  (error) => {
    if (error.response?.status === 401) {
      // Token 过期或无效，清除 token 并跳转到登录页
      localStorage.removeItem('admin_token')
      window.location.href = '/admin/login'
    }
    return Promise.reject(error)
  }
)

export interface LoginRequest {
  email: string
  password: string
  totp_code?: string
}

export interface LoginResponse {
  token: string
  user: {
    email: string
    quota: number
  }
}

export interface User {
  id: number
  email: string
  quota: number
  active: boolean
  is_admin: boolean
  created_at: string
  updated_at: string
}

export interface Domain {
  name: string
  active: boolean
  created_at: string
  updated_at: string
}

export interface Alias {
  from: string
  to: string
  domain: string
  created_at: string
}

export interface Quota {
  limit: number
  used: number
}

export const api = {
  // 认证
  login: (data: LoginRequest): Promise<LoginResponse> => {
    return apiClient.post('/auth/login', data)
  },

  // 初始化
  checkInit: (): Promise<{ needs_init: boolean }> => {
    return apiClient.get('/init/check')
  },
  initSystem: (data: { email: string; password: string }): Promise<{
    success: boolean
    message: string
    user: { email: string }
    password: string
    token?: string
  }> => {
    return apiClient.post('/init', data)
  },

  // 用户管理
  listUsers: (limit = 100, offset = 0): Promise<{ users: User[] }> => {
    return apiClient.get('/users', { params: { limit, offset } })
  },
  getUser: (email: string): Promise<User> => {
    return apiClient.get(`/users/${email}`)
  },
  createUser: (data: { email: string; password: string; quota?: number; active?: boolean; is_admin?: boolean }): Promise<User> => {
    return apiClient.post('/users', data)
  },
  updateUser: (email: string, data: { password?: string; quota?: number; active?: boolean; is_admin?: boolean }): Promise<User> => {
    return apiClient.put(`/users/${email}`, data)
  },
  deleteUser: (email: string): Promise<{ message: string }> => {
    return apiClient.delete(`/users/${email}`)
  },

  // 域名管理
  listDomains: (): Promise<{ domains: Domain[] }> => {
    return apiClient.get('/domains')
  },
  getDomain: (name: string): Promise<Domain> => {
    return apiClient.get(`/domains/${name}`)
  },
  createDomain: (data: { name: string; active?: boolean }): Promise<Domain> => {
    return apiClient.post('/domains', data)
  },
  updateDomain: (name: string, data: { name?: string; active?: boolean }): Promise<Domain> => {
    return apiClient.put(`/domains/${name}`, data)
  },
  deleteDomain: (name: string): Promise<{ message: string }> => {
    return apiClient.delete(`/domains/${name}`)
  },

  // 别名管理
  listAliases: (domain?: string): Promise<{ aliases: Alias[] }> => {
    return apiClient.get('/aliases', { params: { domain } })
  },
  createAlias: (data: { from: string; to: string; domain: string }): Promise<Alias> => {
    return apiClient.post('/aliases', data)
  },
  deleteAlias: (from: string): Promise<{ message: string }> => {
    return apiClient.delete(`/aliases/${from}`)
  },

  // 配额管理
  getQuota: (email: string): Promise<Quota> => {
    return apiClient.get(`/users/${email}/quota`)
  },
  updateQuota: (email: string, data: { limit: number }): Promise<Quota> => {
    return apiClient.put(`/users/${email}/quota`, data)
  }
}

