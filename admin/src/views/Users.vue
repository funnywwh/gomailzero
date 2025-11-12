<template>
  <div class="users-page">
    <div class="page-header">
      <h1>用户管理</h1>
      <button @click="showCreateModal = true" class="btn-primary">+ 创建用户</button>
    </div>

    <div class="table-container">
      <table class="data-table">
        <thead>
          <tr>
            <th>邮箱</th>
            <th>配额</th>
            <th>状态</th>
            <th>管理员</th>
            <th>创建时间</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="loading">
            <td colspan="6" class="text-center">加载中...</td>
          </tr>
          <tr v-else-if="users.length === 0">
            <td colspan="6" class="text-center">暂无用户</td>
          </tr>
          <tr v-else v-for="user in users" :key="user.id">
            <td>{{ user.email }}</td>
            <td>{{ formatQuota(user.quota) }}</td>
            <td>
              <span :class="['status-badge', user.active ? 'active' : 'inactive']">
                {{ user.active ? '启用' : '禁用' }}
              </span>
            </td>
            <td>
              <span :class="['status-badge', user.is_admin ? 'admin' : 'user']">
                {{ user.is_admin ? '是' : '否' }}
              </span>
            </td>
            <td>{{ formatDate(user.created_at) }}</td>
            <td>
              <button @click="editUser(user)" class="btn-sm">编辑</button>
              <button @click="manageQuota(user)" class="btn-sm">配额</button>
              <button @click="deleteUser(user)" class="btn-sm btn-danger">删除</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- 创建/编辑用户模态框 -->
    <div v-if="showCreateModal || editingUser" class="modal-overlay" @click="closeModal">
      <div class="modal" @click.stop>
        <h2>{{ editingUser ? '编辑用户' : '创建用户' }}</h2>
        <form @submit.prevent="saveUser" class="modal-form">
          <div class="form-group">
            <label>邮箱 *</label>
            <input
              v-model="userForm.email"
              type="email"
              required
              :disabled="!!editingUser"
              class="form-input"
            />
          </div>
          <div class="form-group">
            <label>密码 {{ editingUser ? '(留空不修改)' : '*' }}</label>
            <input
              v-model="userForm.password"
              type="password"
              :required="!editingUser"
              class="form-input"
            />
          </div>
          <div class="form-group">
            <label>配额 (MB, 0=无限制)</label>
            <input
              v-model.number="userForm.quota"
              type="number"
              min="0"
              class="form-input"
            />
          </div>
          <div class="form-group">
            <label>
              <input
                v-model="userForm.active"
                type="checkbox"
                class="form-checkbox"
              />
              启用
            </label>
          </div>
          <div class="form-group">
            <label>
              <input
                v-model="userForm.isAdmin"
                type="checkbox"
                class="form-checkbox"
              />
              管理员
            </label>
          </div>
          <div v-if="error" class="error-message">{{ error }}</div>
          <div class="modal-actions">
            <button type="button" @click="closeModal" class="btn-secondary">取消</button>
            <button type="submit" :disabled="saving" class="btn-primary">
              {{ saving ? '保存中...' : '保存' }}
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- 配额管理模态框 -->
    <div v-if="quotaUser" class="modal-overlay" @click="quotaUser = null">
      <div class="modal" @click.stop>
        <h2>管理配额 - {{ quotaUser.email }}</h2>
        <form @submit.prevent="saveQuota" class="modal-form">
          <div class="form-group">
            <label>配额限制 (MB, 0=无限制)</label>
            <input
              v-model.number="quotaForm.limit"
              type="number"
              min="0"
              required
              class="form-input"
            />
          </div>
          <div v-if="quotaInfo" class="quota-info">
            <p>已使用: {{ formatSize(quotaInfo.used) }}</p>
            <p>限制: {{ formatQuota(quotaForm.limit) }}</p>
          </div>
          <div v-if="error" class="error-message">{{ error }}</div>
          <div class="modal-actions">
            <button type="button" @click="quotaUser = null" class="btn-secondary">取消</button>
            <button type="submit" :disabled="saving" class="btn-primary">
              {{ saving ? '保存中...' : '保存' }}
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { api, type User, type Quota } from '../api'

const users = ref<User[]>([])
const loading = ref(false)
const error = ref('')
const showCreateModal = ref(false)
const editingUser = ref<User | null>(null)
const quotaUser = ref<User | null>(null)
const quotaInfo = ref<Quota | null>(null)
const saving = ref(false)

const userForm = ref({
  email: '',
  password: '',
  quota: 0,
  active: true,
  isAdmin: false
})

const quotaForm = ref({
  limit: 0
})

const loadUsers = async () => {
  loading.value = true
  error.value = ''
  try {
    const response = await api.listUsers()
    users.value = response.users
  } catch (err: any) {
    error.value = err.response?.data?.error || '加载用户失败'
  } finally {
    loading.value = false
  }
}

const editUser = (user: User) => {
  editingUser.value = user
  userForm.value = {
    email: user.email,
    password: '',
    quota: user.quota / (1024 * 1024), // 转换为 MB
    active: user.active,
    isAdmin: user.is_admin || false
  }
}

const saveUser = async () => {
  saving.value = true
  error.value = ''
  try {
    const data: any = {
      quota: userForm.value.quota * 1024 * 1024, // 转换为字节
      active: userForm.value.active,
      is_admin: userForm.value.isAdmin
    }
    if (userForm.value.password) {
      data.password = userForm.value.password
    }

    if (editingUser.value) {
      await api.updateUser(editingUser.value.email, data)
    } else {
      await api.createUser({
        email: userForm.value.email,
        password: userForm.value.password,
        quota: data.quota,
        active: data.active,
        is_admin: data.is_admin
      })
    }
    closeModal()
    await loadUsers()
  } catch (err: any) {
    error.value = err.response?.data?.error || '保存失败'
  } finally {
    saving.value = false
  }
}

const deleteUser = async (user: User) => {
  if (!confirm(`确定要删除用户 ${user.email} 吗？`)) {
    return
  }
  try {
    await api.deleteUser(user.email)
    await loadUsers()
  } catch (err: any) {
    error.value = err.response?.data?.error || '删除失败'
  }
}

const manageQuota = async (user: User) => {
  quotaUser.value = user
  quotaForm.value.limit = user.quota / (1024 * 1024)
  try {
    const quota = await api.getQuota(user.email)
    quotaInfo.value = quota
  } catch (err: any) {
    console.error('获取配额信息失败:', err)
  }
}

const saveQuota = async () => {
  if (!quotaUser.value) return
  saving.value = true
  error.value = ''
  try {
    await api.updateQuota(quotaUser.value.email, {
      limit: quotaForm.value.limit * 1024 * 1024
    })
    quotaUser.value = null
    await loadUsers()
  } catch (err: any) {
    error.value = err.response?.data?.error || '保存配额失败'
  } finally {
    saving.value = false
  }
}

const closeModal = () => {
  showCreateModal.value = false
  editingUser.value = null
  userForm.value = {
    email: '',
    password: '',
    quota: 0,
    active: true,
    isAdmin: false
  }
  error.value = ''
}

const formatQuota = (quota: number) => {
  if (quota === 0) return '无限制'
  return formatSize(quota)
}

const formatSize = (bytes: number) => {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(2)} KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(2)} MB`
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`
}

const formatDate = (dateStr: string) => {
  return new Date(dateStr).toLocaleString('zh-CN')
}

onMounted(() => {
  loadUsers()
})
</script>

<style scoped>
.users-page {
  padding: 2rem;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 2rem;
}

.page-header h1 {
  font-size: 1.5rem;
  color: #333;
}

.btn-primary {
  padding: 0.75rem 1.5rem;
  background: #667eea;
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.875rem;
}

.btn-primary:hover {
  background: #5568d3;
}

.table-container {
  background: white;
  border-radius: 8px;
  overflow: hidden;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}

.data-table {
  width: 100%;
  border-collapse: collapse;
}

.data-table th {
  background: #f8f9fa;
  padding: 1rem;
  text-align: left;
  font-weight: 600;
  color: #666;
  border-bottom: 2px solid #e0e0e0;
}

.data-table td {
  padding: 1rem;
  border-bottom: 1px solid #e0e0e0;
}

.data-table tr:hover {
  background: #f8f9fa;
}

.status-badge {
  padding: 0.25rem 0.75rem;
  border-radius: 12px;
  font-size: 0.875rem;
}

.status-badge.active {
  background: #d4edda;
  color: #155724;
}

.status-badge.inactive {
  background: #f8d7da;
  color: #721c24;
}

.status-badge.admin {
  background: #d1ecf1;
  color: #0c5460;
}

.status-badge.user {
  background: #e2e3e5;
  color: #383d41;
}

.btn-sm {
  padding: 0.375rem 0.75rem;
  margin-right: 0.5rem;
  background: #667eea;
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.875rem;
}

.btn-sm:hover {
  background: #5568d3;
}

.btn-sm.btn-danger {
  background: #e74c3c;
}

.btn-sm.btn-danger:hover {
  background: #c0392b;
}

.text-center {
  text-align: center;
  padding: 2rem;
  color: #999;
}

.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  justify-content: center;
  align-items: center;
  z-index: 1000;
}

.modal {
  background: white;
  padding: 2rem;
  border-radius: 8px;
  width: 90%;
  max-width: 500px;
  max-height: 90vh;
  overflow-y: auto;
}

.modal h2 {
  margin-bottom: 1.5rem;
  color: #333;
}

.modal-form {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.form-group label {
  font-weight: 500;
  color: #666;
}

.form-input {
  padding: 0.75rem;
  border: 1px solid #ddd;
  border-radius: 4px;
  font-size: 1rem;
}

.form-input:focus {
  outline: none;
  border-color: #667eea;
}

.form-checkbox {
  margin-right: 0.5rem;
}

.modal-actions {
  display: flex;
  gap: 1rem;
  justify-content: flex-end;
  margin-top: 1rem;
}

.btn-secondary {
  padding: 0.75rem 1.5rem;
  background: #6c757d;
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
}

.btn-secondary:hover {
  background: #5a6268;
}

.error-message {
  color: #e74c3c;
  font-size: 0.875rem;
}

.quota-info {
  padding: 1rem;
  background: #f8f9fa;
  border-radius: 4px;
  margin: 1rem 0;
}

.quota-info p {
  margin: 0.5rem 0;
  color: #666;
}
</style>

