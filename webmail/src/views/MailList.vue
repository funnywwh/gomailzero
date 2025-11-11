<template>
  <div class="mail-list-container">
    <header class="header">
      <h1>邮件</h1>
      <button @click="handleCompose" class="compose-btn">写邮件</button>
      <button @click="handleLogout" class="logout-btn">退出</button>
    </header>
    <div class="content">
      <aside class="sidebar">
        <nav>
          <a href="#" @click.prevent="setFolder('INBOX')" :class="{ active: currentFolder === 'INBOX' }">
            收件箱
          </a>
          <a href="#" @click.prevent="setFolder('Sent')" :class="{ active: currentFolder === 'Sent' }">
            已发送
          </a>
          <a href="#" @click.prevent="setFolder('Drafts')" :class="{ active: currentFolder === 'Drafts' }">
            草稿
          </a>
          <a href="#" @click.prevent="setFolder('Trash')" :class="{ active: currentFolder === 'Trash' }">
            垃圾箱
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

const setFolder = (folder: string) => {
  currentFolder.value = folder
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

onMounted(() => {
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

.compose-btn,
.logout-btn {
  padding: 0.5rem 1rem;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.875rem;
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

