<template>
  <div class="mail-list-container">
    <header class="header">
      <h1>邮件</h1>
      <div class="header-actions">
        <input
          v-model="searchQuery"
          @keyup.enter="handleSearch"
          type="text"
          placeholder="搜索邮件..."
          class="search-input"
        />
        <button @click="handleSearch" class="search-btn">搜索</button>
        <button @click="handleCompose" class="compose-btn">写邮件</button>
        <button @click="handleLogout" class="logout-btn">退出</button>
      </div>
    </header>
    <div class="content">
      <aside class="sidebar">
        <nav>
          <a
            v-for="folder in folders"
            :key="folder"
            href="#"
            @click.prevent="setFolder(folder)"
            :class="{ active: currentFolder === folder }"
          >
            {{ getFolderName(folder) }}
          </a>
        </nav>
      </aside>
      <main class="mail-list">
        <div v-if="loading" class="loading">加载中...</div>
        <div v-else-if="error" class="error">{{ error }}</div>
        <div v-else-if="mails.length === 0" class="empty">暂无邮件</div>
        <ul v-else class="mail-items">
          <li
            v-for="mail in mails"
            :key="mail.id"
            class="mail-item"
            :class="{ unread: !mail.flags?.includes('\\Seen') }"
            @click="viewMail(mail.id)"
          >
            <div class="mail-from">{{ mail.from }}</div>
            <div class="mail-subject">{{ mail.subject || '(无主题)' }}</div>
            <div class="mail-date">{{ formatDate(mail.received_at) }}</div>
          </li>
        </ul>
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api'

const router = useRouter()

const mails = ref<any[]>([])
const currentFolder = ref('INBOX')
const loading = ref(false)
const error = ref('')
const searchQuery = ref('')
const folders = ref<string[]>([])

const loadMails = async () => {
  loading.value = true
  error.value = ''

  try {
    const response = await api.getMails(currentFolder.value)
    mails.value = response.mails || []
  } catch (err: any) {
    error.value = err.response?.data?.error || '加载邮件失败'
  } finally {
    loading.value = false
  }
}

const loadFolders = async () => {
  try {
    const response = await api.listFolders()
    folders.value = response.folders || []
  } catch (err: any) {
    console.error('加载文件夹失败:', err)
  }
}

const handleSearch = async () => {
  if (!searchQuery.value.trim()) {
    loadMails()
    return
  }

  loading.value = true
  error.value = ''

  try {
    const response = await api.searchMails(searchQuery.value, currentFolder.value)
    mails.value = response.mails || []
  } catch (err: any) {
    error.value = err.response?.data?.error || '搜索失败'
  } finally {
    loading.value = false
  }
}

const setFolder = (folder: string) => {
  currentFolder.value = folder
  searchQuery.value = ''
  loadMails()
}

const viewMail = (id: string) => {
  router.push(`/mails/${id}`)
}

const handleCompose = () => {
  router.push('/compose')
}

const handleLogout = () => {
  localStorage.removeItem('token')
  router.push('/login')
}

const formatDate = (date: string) => {
  const d = new Date(date)
  return d.toLocaleDateString('zh-CN', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  })
}

const getFolderName = (folder: string) => {
  const names: Record<string, string> = {
    INBOX: '收件箱',
    Sent: '已发送',
    Drafts: '草稿',
    Trash: '垃圾箱',
    Spam: '垃圾邮件'
  }
  return names[folder] || folder
}

onMounted(() => {
  loadFolders()
  loadMails()
})
</script>

<style scoped>
.mail-list-container {
  display: flex;
  flex-direction: column;
  height: 100vh;
}

.header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1rem 2rem;
  background: #fff;
  border-bottom: 1px solid #e0e0e0;
}

.header h1 {
  font-size: 1.5rem;
  color: #333;
}

.header-actions {
  display: flex;
  gap: 0.5rem;
  align-items: center;
}

.search-input {
  padding: 0.5rem;
  border: 1px solid #ddd;
  border-radius: 4px;
  font-size: 0.875rem;
  width: 200px;
}

.search-btn,
.compose-btn,
.logout-btn {
  padding: 0.5rem 1rem;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.875rem;
}

.search-btn {
  background: #f5f5f5;
  color: #666;
}

.compose-btn {
  background: #667eea;
  color: white;
  margin-right: 0.5rem;
}

.logout-btn {
  background: #f5f5f5;
  color: #666;
}

.content {
  display: flex;
  flex: 1;
  overflow: hidden;
}

.sidebar {
  width: 200px;
  background: #f9f9f9;
  border-right: 1px solid #e0e0e0;
  padding: 1rem;
}

.sidebar nav a {
  display: block;
  padding: 0.75rem;
  color: #666;
  text-decoration: none;
  border-radius: 4px;
  margin-bottom: 0.5rem;
}

.sidebar nav a:hover,
.sidebar nav a.active {
  background: #667eea;
  color: white;
}

.mail-list {
  flex: 1;
  overflow-y: auto;
  background: white;
}

.mail-items {
  list-style: none;
}

.mail-item {
  padding: 1rem;
  border-bottom: 1px solid #e0e0e0;
  cursor: pointer;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.mail-item:hover {
  background: #f5f5f5;
}

.mail-item.unread {
  font-weight: 500;
}

.mail-from {
  flex: 1;
  color: #333;
}

.mail-subject {
  flex: 2;
  color: #666;
  margin-left: 1rem;
}

.mail-date {
  flex: 0 0 auto;
  color: #999;
  font-size: 0.875rem;
}

.loading,
.error,
.empty {
  padding: 2rem;
  text-align: center;
  color: #666;
}

.error {
  color: #e74c3c;
}
</style>

