<template>
  <div class="domains-page">
    <div class="page-header">
      <h1>域名管理</h1>
      <button @click="showCreateModal = true" class="btn-primary">+ 创建域名</button>
    </div>

    <div class="table-container">
      <table class="data-table">
        <thead>
          <tr>
            <th>域名</th>
            <th>状态</th>
            <th>创建时间</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="loading">
            <td colspan="4" class="text-center">加载中...</td>
          </tr>
          <tr v-else-if="domains.length === 0">
            <td colspan="4" class="text-center">暂无域名</td>
          </tr>
          <tr v-else v-for="domain in domains" :key="domain.name">
            <td>{{ domain.name }}</td>
            <td>
              <span :class="['status-badge', domain.active ? 'active' : 'inactive']">
                {{ domain.active ? '启用' : '禁用' }}
              </span>
            </td>
            <td>{{ formatDate(domain.created_at) }}</td>
            <td>
              <button @click="editDomain(domain)" class="btn-sm">编辑</button>
              <button @click="deleteDomain(domain)" class="btn-sm btn-danger">删除</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- 创建/编辑域名模态框 -->
    <div v-if="showCreateModal || editingDomain" class="modal-overlay" @click="closeModal">
      <div class="modal" @click.stop>
        <h2>{{ editingDomain ? '编辑域名' : '创建域名' }}</h2>
        <form @submit.prevent="saveDomain" class="modal-form">
          <div class="form-group">
            <label>域名 *</label>
            <input
              v-model="domainForm.name"
              type="text"
              required
              :disabled="!!editingDomain"
              placeholder="example.com"
              class="form-input"
            />
          </div>
          <div class="form-group">
            <label>
              <input
                v-model="domainForm.active"
                type="checkbox"
                class="form-checkbox"
              />
              启用
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
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { api, type Domain } from '../api'

const domains = ref<Domain[]>([])
const loading = ref(false)
const error = ref('')
const showCreateModal = ref(false)
const editingDomain = ref<Domain | null>(null)
const saving = ref(false)

const domainForm = ref({
  name: '',
  active: true
})

const loadDomains = async () => {
  loading.value = true
  error.value = ''
  try {
    const response = await api.listDomains()
    domains.value = response.domains
  } catch (err: any) {
    error.value = err.response?.data?.error || '加载域名失败'
  } finally {
    loading.value = false
  }
}

const editDomain = (domain: Domain) => {
  editingDomain.value = domain
  domainForm.value = {
    name: domain.name,
    active: domain.active
  }
}

const saveDomain = async () => {
  saving.value = true
  error.value = ''
  try {
    if (editingDomain.value) {
      await api.updateDomain(editingDomain.value.name, {
        active: domainForm.value.active
      })
    } else {
      await api.createDomain({
        name: domainForm.value.name,
        active: domainForm.value.active
      })
    }
    closeModal()
    await loadDomains()
  } catch (err: any) {
    error.value = err.response?.data?.error || '保存失败'
  } finally {
    saving.value = false
  }
}

const deleteDomain = async (domain: Domain) => {
  if (!confirm(`确定要删除域名 ${domain.name} 吗？`)) {
    return
  }
  try {
    await api.deleteDomain(domain.name)
    await loadDomains()
  } catch (err: any) {
    error.value = err.response?.data?.error || '删除失败'
  }
}

const closeModal = () => {
  showCreateModal.value = false
  editingDomain.value = null
  domainForm.value = {
    name: '',
    active: true
  }
  error.value = ''
}

const formatDate = (dateStr: string) => {
  return new Date(dateStr).toLocaleString('zh-CN')
}

onMounted(() => {
  loadDomains()
})
</script>

<style scoped>
.domains-page {
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
</style>

