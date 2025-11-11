<template>
  <div class="mail-view-container">
      <header class="header">
      <button @click="goBack" class="back-btn">← 返回</button>
      <div class="header-actions">
        <button @click="handleMarkRead" class="mark-btn">标记为已读</button>
        <button @click="handleReply" class="reply-btn">回复</button>
        <button @click="handleForward" class="forward-btn">转发</button>
        <button @click="handleDelete" class="delete-btn">删除</button>
      </div>
    </header>
    <div v-if="loading" class="loading">加载中...</div>
    <div v-else-if="error" class="error">{{ error }}</div>
    <div v-else-if="mail" class="mail-content">
      <div class="mail-header">
        <h2>{{ mail.subject || '(无主题)' }}</h2>
        <div class="mail-meta">
          <div><strong>发件人:</strong> {{ mail.from }}</div>
          <div><strong>收件人:</strong> {{ mail.to?.join(', ') }}</div>
          <div v-if="mail.cc?.length"><strong>抄送:</strong> {{ mail.cc.join(', ') }}</div>
          <div><strong>时间:</strong> {{ formatDate(mail.received_at) }}</div>
        </div>
      </div>
      <div class="mail-body" v-html="mail.body"></div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { api } from '../api'

const router = useRouter()
const route = useRoute()

const mail = ref<any>(null)
const loading = ref(false)
const error = ref('')

const loadMail = async () => {
  loading.value = true
  error.value = ''

  try {
    const id = route.params.id as string
    mail.value = await api.getMail(id)
    // 自动标记为已读
    if (mail.value && !mail.value.flags?.includes('\\Seen')) {
      await api.updateMailFlags(id, [...(mail.value.flags || []), '\\Seen'])
    }
  } catch (err: any) {
    error.value = err.response?.data?.error || '加载邮件失败'
  } finally {
    loading.value = false
  }
}

const handleMarkRead = async () => {
  if (!mail.value) return

  try {
    const id = route.params.id as string
    const flags = mail.value.flags || []
    if (flags.includes('\\Seen')) {
      await api.updateMailFlags(id, flags.filter((f) => f !== '\\Seen'))
    } else {
      await api.updateMailFlags(id, [...flags, '\\Seen'])
    }
    await loadMail()
  } catch (err: any) {
    error.value = err.response?.data?.error || '更新标记失败'
  }
}

const handleReply = () => {
  if (!mail.value) return
  router.push({
    path: '/compose',
    query: {
      reply: mail.value.id,
      subject: mail.value.subject?.startsWith('Re:') ? mail.value.subject : `Re: ${mail.value.subject || ''}`
    }
  })
}

const handleForward = () => {
  if (!mail.value) return
  router.push({
    path: '/compose',
    query: {
      forward: mail.value.id,
      subject: mail.value.subject?.startsWith('Fwd:') ? mail.value.subject : `Fwd: ${mail.value.subject || ''}`
    }
  })
}

const goBack = () => {
  router.push('/mails')
}

const handleDelete = async () => {
  if (!confirm('确定要删除这封邮件吗？')) {
    return
  }

  try {
    const id = route.params.id as string
    await api.deleteMail(id)
    router.push('/mails')
  } catch (err: any) {
    error.value = err.response?.data?.error || '删除失败'
  }
}

const formatDate = (date: string) => {
  const d = new Date(date)
  return d.toLocaleString('zh-CN')
}

onMounted(() => {
  loadMail()
})
</script>

<style scoped>
.mail-view-container {
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

.header-actions {
  display: flex;
  gap: 0.5rem;
}

.back-btn,
.mark-btn,
.reply-btn,
.forward-btn,
.delete-btn {
  padding: 0.5rem 1rem;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.875rem;
}

.mark-btn {
  background: #f5f5f5;
  color: #666;
}

.reply-btn,
.forward-btn {
  background: #667eea;
  color: white;
}

.back-btn {
  background: #f5f5f5;
  color: #666;
}

.delete-btn {
  background: #e74c3c;
  color: white;
}

.mail-content {
  flex: 1;
  overflow-y: auto;
  padding: 2rem;
  background: white;
}

.mail-header {
  margin-bottom: 2rem;
  padding-bottom: 1rem;
  border-bottom: 1px solid #e0e0e0;
}

.mail-header h2 {
  margin-bottom: 1rem;
  color: #333;
}

.mail-meta {
  color: #666;
  font-size: 0.875rem;
}

.mail-meta div {
  margin-bottom: 0.5rem;
}

.mail-body {
  line-height: 1.6;
  color: #333;
}

.mail-body-html {
  max-width: 100%;
  overflow-x: auto;
}

.mail-body-html :deep(img) {
  max-width: 100%;
  height: auto;
}

.mail-body-text {
  white-space: pre-wrap;
  word-wrap: break-word;
}

.mail-body-empty {
  color: #999;
  font-style: italic;
}

.loading,
.error {
  padding: 2rem;
  text-align: center;
}

.error {
  color: #e74c3c;
}
</style>

