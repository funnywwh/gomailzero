<template>
  <div class="compose-container">
    <header class="header">
      <h1>写邮件</h1>
      <div>
        <button @click="handleSave" class="save-btn">保存草稿</button>
        <button @click="handleSend" :disabled="sending" class="send-btn">
          {{ sending ? '发送中...' : '发送' }}
        </button>
        <button @click="goBack" class="cancel-btn">取消</button>
      </div>
    </header>
    <form @submit.prevent="handleSend" class="compose-form">
      <div class="form-group">
        <label>收件人</label>
        <input v-model="form.to" type="text" required placeholder="email@example.com" />
      </div>
      <div class="form-group">
        <label>主题</label>
        <input v-model="form.subject" type="text" placeholder="邮件主题" />
      </div>
      <div class="form-group">
        <label>正文</label>
        <textarea v-model="form.body" rows="20" required placeholder="邮件正文..."></textarea>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
    </form>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api'

const router = useRouter()

const form = ref({
  to: '',
  subject: '',
  body: ''
})

const sending = ref(false)
const error = ref('')

const handleSend = async () => {
  sending.value = true
  error.value = ''

  try {
    const toList = form.value.to.split(',').map((email) => email.trim())
    await api.sendMail({
      to: toList,
      subject: form.value.subject,
      body: form.value.body
    })
    router.push('/mails')
  } catch (err: any) {
    error.value = err.response?.data?.error || '发送失败'
  } finally {
    sending.value = false
  }
}

const handleSave = async () => {
  if (!form.value.to && !form.value.subject && !form.value.body) {
    alert('草稿内容不能为空')
    return
  }

  try {
    const toList = form.value.to ? form.value.to.split(',').map((email) => email.trim()) : []
    await api.saveDraft({
      to: toList,
      subject: form.value.subject,
      body: form.value.body
    })
    alert('草稿已保存')
  } catch (err: any) {
    error.value = err.response?.data?.error || '保存草稿失败'
  }
}

const goBack = () => {
  router.push('/mails')
}

onMounted(() => {
  // 处理回复和转发
  const replyId = route.query.reply as string
  const forwardId = route.query.forward as string

  if (replyId) {
    // 加载原邮件并设置回复内容
    api.getMail(replyId).then((mail) => {
      form.value.to = mail.from
      form.value.subject = (route.query.subject as string) || `Re: ${mail.subject || ''}`
      form.value.body = `\n\n--- 原始邮件 ---\n发件人: ${mail.from}\n日期: ${new Date(mail.received_at).toLocaleString()}\n主题: ${mail.subject || ''}\n\n${mail.body ? new TextDecoder().decode(mail.body) : ''}`
    }).catch((err) => {
      console.error('加载邮件失败:', err)
    })
  } else if (forwardId) {
    // 加载原邮件并设置转发内容
    api.getMail(forwardId).then((mail) => {
      form.value.subject = (route.query.subject as string) || `Fwd: ${mail.subject || ''}`
      form.value.body = `\n\n--- 转发邮件 ---\n发件人: ${mail.from}\n日期: ${new Date(mail.received_at).toLocaleString()}\n主题: ${mail.subject || ''}\n收件人: ${mail.to?.join(', ') || ''}\n\n${mail.body ? new TextDecoder().decode(mail.body) : ''}`
    }).catch((err) => {
      console.error('加载邮件失败:', err)
    })
  }
})
</script>

<style scoped>
.compose-container {
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

.save-btn,
.send-btn,
.cancel-btn {
  padding: 0.5rem 1rem;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.875rem;
  margin-left: 0.5rem;
}

.save-btn {
  background: #f5f5f5;
  color: #666;
}

.send-btn {
  background: #667eea;
  color: white;
}

.send-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.cancel-btn {
  background: #e74c3c;
  color: white;
}

.compose-form {
  flex: 1;
  overflow-y: auto;
  padding: 2rem;
  background: white;
}

.form-group {
  margin-bottom: 1.5rem;
}

label {
  display: block;
  margin-bottom: 0.5rem;
  color: #666;
  font-weight: 500;
}

input,
textarea {
  width: 100%;
  padding: 0.75rem;
  border: 1px solid #ddd;
  border-radius: 4px;
  font-size: 1rem;
  font-family: inherit;
}

input:focus,
textarea:focus {
  outline: none;
  border-color: #667eea;
}

textarea {
  resize: vertical;
  min-height: 300px;
}

.error {
  color: #e74c3c;
  margin-top: 1rem;
}
</style>

