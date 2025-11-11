<template>
  <div class="init-container">
    <div class="init-box">
      <h1>GoMailZero 初始化</h1>
      <p class="description">欢迎使用 GoMailZero！请创建管理员账户以开始使用。</p>
      
      <form v-if="!success" @submit.prevent="handleInit">
        <div class="form-group">
          <label for="email">管理员邮箱</label>
          <input
            id="email"
            v-model="email"
            type="email"
            required
            placeholder="admin@example.com"
          />
        </div>
        <div class="form-group">
          <label for="password">管理员密码</label>
          <input
            id="password"
            v-model="password"
            type="password"
            required
            placeholder="至少 8 位字符"
            minlength="8"
          />
          <small class="hint">密码长度至少为 8 位</small>
        </div>
        <div v-if="error" class="error">{{ error }}</div>
        <button type="submit" :disabled="loading">
          {{ loading ? '初始化中...' : '初始化系统' }}
        </button>
      </form>

      <div v-else class="success-box">
        <div class="success-icon">✓</div>
        <h2>初始化成功！</h2>
        <p class="success-message">系统已成功初始化，以下是您的登录信息：</p>
        <div class="credentials">
          <div class="credential-item">
            <label>邮箱：</label>
            <code>{{ initResult.email }}</code>
            <button @click="copyToClipboard(initResult.email)" class="copy-btn">复制</button>
          </div>
          <div class="credential-item">
            <label>密码：</label>
            <code class="password-display">{{ initResult.password }}</code>
            <button @click="copyToClipboard(initResult.password)" class="copy-btn">复制</button>
          </div>
        </div>
        <div class="warning">
          <strong>⚠️ 重要提示：</strong>
          <p>请妥善保存以上登录信息，此页面关闭后将无法再次查看密码。</p>
          <p>建议立即登录并修改密码。</p>
        </div>
        <button @click="handleLogin" class="login-btn">立即登录</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api'

const router = useRouter()

const email = ref('')
const password = ref('')
const loading = ref(false)
const error = ref('')
const success = ref(false)
const initResult = ref<{ email: string; password: string; token?: string }>({
  email: '',
  password: ''
})

const handleInit = async () => {
  loading.value = true
  error.value = ''

  try {
    const response = await api.initSystem({
      email: email.value,
      password: password.value
    })

    if (response.success) {
      initResult.value = {
        email: response.user.email,
        password: response.password,
        token: response.token
      }
      success.value = true

      // 如果返回了 token，保存并准备自动登录
      if (response.token) {
        localStorage.setItem('token', response.token)
      }
    }
  } catch (err: any) {
    error.value = err.response?.data?.error || '初始化失败'
  } finally {
    loading.value = false
  }
}

const copyToClipboard = async (text: string) => {
  try {
    await navigator.clipboard.writeText(text)
    alert('已复制到剪贴板')
  } catch (err) {
    // 降级方案
    const textarea = document.createElement('textarea')
    textarea.value = text
    document.body.appendChild(textarea)
    textarea.select()
    document.execCommand('copy')
    document.body.removeChild(textarea)
    alert('已复制到剪贴板')
  }
}

const handleLogin = () => {
  if (initResult.value.token) {
    // 已有 token，直接跳转
    router.push('/mails')
  } else {
    // 跳转到登录页面
    router.push('/login')
  }
}
</script>

<style scoped>
.init-container {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 100vh;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  padding: 2rem;
}

.init-box {
  background: white;
  padding: 2rem;
  border-radius: 8px;
  box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
  width: 100%;
  max-width: 500px;
}

h1 {
  text-align: center;
  margin-bottom: 0.5rem;
  color: #333;
}

.description {
  text-align: center;
  color: #666;
  margin-bottom: 2rem;
  font-size: 0.9rem;
}

.form-group {
  margin-bottom: 1rem;
}

label {
  display: block;
  margin-bottom: 0.5rem;
  color: #666;
  font-weight: 500;
}

input {
  width: 100%;
  padding: 0.75rem;
  border: 1px solid #ddd;
  border-radius: 4px;
  font-size: 1rem;
  box-sizing: border-box;
}

input:focus {
  outline: none;
  border-color: #667eea;
}

.hint {
  display: block;
  margin-top: 0.25rem;
  color: #999;
  font-size: 0.875rem;
}

button {
  width: 100%;
  padding: 0.75rem;
  background: #667eea;
  color: white;
  border: none;
  border-radius: 4px;
  font-size: 1rem;
  font-weight: 500;
  cursor: pointer;
  margin-top: 1rem;
}

button:hover:not(:disabled) {
  background: #5568d3;
}

button:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.error {
  color: #e74c3c;
  margin-top: 0.5rem;
  font-size: 0.875rem;
  padding: 0.5rem;
  background: #fee;
  border-radius: 4px;
}

.success-box {
  text-align: center;
}

.success-icon {
  width: 64px;
  height: 64px;
  margin: 0 auto 1rem;
  background: #4caf50;
  color: white;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 2rem;
  font-weight: bold;
}

.success-box h2 {
  color: #333;
  margin-bottom: 1rem;
}

.success-message {
  color: #666;
  margin-bottom: 1.5rem;
}

.credentials {
  background: #f5f5f5;
  padding: 1.5rem;
  border-radius: 4px;
  margin-bottom: 1.5rem;
  text-align: left;
}

.credential-item {
  display: flex;
  align-items: center;
  margin-bottom: 1rem;
  gap: 0.5rem;
}

.credential-item:last-child {
  margin-bottom: 0;
}

.credential-item label {
  min-width: 60px;
  font-weight: 500;
  color: #666;
  margin: 0;
}

.credential-item code {
  flex: 1;
  padding: 0.5rem;
  background: white;
  border: 1px solid #ddd;
  border-radius: 4px;
  font-family: 'Courier New', monospace;
  font-size: 0.9rem;
  word-break: break-all;
}

.password-display {
  font-weight: bold;
  color: #667eea;
}

.copy-btn {
  padding: 0.5rem 1rem;
  background: #667eea;
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.875rem;
  margin: 0;
  width: auto;
  min-width: 60px;
}

.copy-btn:hover {
  background: #5568d3;
}

.warning {
  background: #fff3cd;
  border: 1px solid #ffc107;
  border-radius: 4px;
  padding: 1rem;
  margin-bottom: 1.5rem;
  text-align: left;
}

.warning strong {
  color: #856404;
  display: block;
  margin-bottom: 0.5rem;
}

.warning p {
  color: #856404;
  margin: 0.5rem 0;
  font-size: 0.875rem;
}

.login-btn {
  background: #4caf50;
}

.login-btn:hover {
  background: #45a049;
}
</style>

