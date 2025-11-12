<template>
  <div class="aliases-page">
    <div class="page-header">
      <h1>别名管理</h1>
      <button @click="showCreateModal = true" class="btn-primary">+ 创建别名</button>
    </div>

    <div class="table-container">
      <table class="data-table">
        <thead>
          <tr>
            <th>来源邮箱</th>
            <th>目标邮箱</th>
            <th>域名</th>
            <th>创建时间</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="loading">
            <td colspan="5" class="text-center">加载中...</td>
          </tr>
          <tr v-else-if="aliases.length === 0">
            <td colspan="5" class="text-center">暂无别名</td>
          </tr>
          <tr v-else v-for="alias in aliases" :key="alias.from">
            <td>{{ alias.from }}</td>
            <td>{{ alias.to }}</td>
            <td>{{ alias.domain }}</td>
            <td>{{ formatDate(alias.created_at) }}</td>
            <td>
              <button @click="deleteAlias(alias)" class="btn-sm btn-danger">删除</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- 创建别名模态框 -->
    <div v-if="showCreateModal" class="modal-overlay" @click="closeModal">
      <div class="modal" @click.stop>
        <h2>创建别名</h2>
        <form @submit.prevent="saveAlias" class="modal-form">
          <div class="form-group">
            <label>来源邮箱 *</label>
            <input
              v-model="aliasForm.from"
              type="email"
              required
              placeholder="alias@example.com"
              class="form-input"
            />
          </div>
          <div class="form-group">
            <label>目标邮箱 *</label>
            <input
              v-model="aliasForm.to"
              type="email"
              required
              placeholder="user@example.com"
              class="form-input"
            />
          </div>
          <div class="form-group">
            <label>域名 *</label>
            <input
              v-model="aliasForm.domain"
              type="text"
              required
              placeholder="example.com"
              class="form-input"
            />
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
import { api, type Alias } from '../api'

const aliases = ref<Alias[]>([])
const loading = ref(false)
const error = ref('')
const showCreateModal = ref(false)
const saving = ref(false)

const aliasForm = ref({
  from: '',
  to: '',
  domain: ''
})

const loadAliases = async () => {
  loading.value = true
  error.value = ''
  try {
    const response = await api.listAliases()
    aliases.value = response.aliases
  } catch (err: any) {
    error.value = err.response?.data?.error || '加载别名失败'
  } finally {
    loading.value = false
  }
}

const saveAlias = async () => {
  saving.value = true
  error.value = ''
  try {
    await api.createAlias({
      from: aliasForm.value.from,
      to: aliasForm.value.to,
      domain: aliasForm.value.domain
    })
    closeModal()
    await loadAliases()
  } catch (err: any) {
    error.value = err.response?.data?.error || '创建失败'
  } finally {
    saving.value = false
  }
}

const deleteAlias = async (alias: Alias) => {
  if (!confirm(`确定要删除别名 ${alias.from} -> ${alias.to} 吗？`)) {
    return
  }
  try {
    await api.deleteAlias(alias.from)
    await loadAliases()
  } catch (err: any) {
    error.value = err.response?.data?.error || '删除失败'
  }
}

const closeModal = () => {
  showCreateModal.value = false
  aliasForm.value = {
    from: '',
    to: '',
    domain: ''
  }
  error.value = ''
}

const formatDate = (dateStr: string) => {
  return new Date(dateStr).toLocaleString('zh-CN')
}

onMounted(() => {
  loadAliases()
})
</script>

<style scoped>
.aliases-page {
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

.btn-sm {
  padding: 0.375rem 0.75rem;
  background: #e74c3c;
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.875rem;
}

.btn-sm:hover {
  background: #c0392b;
}

.btn-sm.btn-danger {
  background: #e74c3c;
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

