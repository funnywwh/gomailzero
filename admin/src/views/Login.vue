<template>
  <div class="login-container">
    <div class="login-box">
      <h1>GoMailZero 管理后台</h1>
      <form @submit.prevent="handleLogin" class="login-form">
        <div class="form-group">
          <label>邮箱</label>
          <input
            v-model="form.email"
            type="email"
            required
            placeholder="admin@example.com"
            class="form-input"
          />
        </div>
        <div class="form-group">
          <label>密码</label>
          <input
            v-model="form.password"
            type="password"
            required
            placeholder="请输入密码"
            class="form-input"
          />
        </div>
        <div v-if="requires2FA" class="form-group">
          <label>TOTP 代码</label>
          <input
            v-model="form.totpCode"
            type="text"
            required
            placeholder="请输入 TOTP 代码"
            class="form-input"
            maxlength="6"
          />
        </div>
        <div v-if="error" class="error-message">{{ error }}</div>
        <button type="submit" :disabled="loading" class="login-btn">
          {{ loading ? '登录中...' : '登录' }}
        </button>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api'

const router = useRouter()

const form = ref({
  email: '',
  password: '',
  totpCode: ''
})

const loading = ref(false)
const error = ref('')
const requires2FA = ref(false)

const handleLogin = async () => {
  loading.value = true
  error.value = ''

  try {
    const response = await api.login({
      email: form.value.email,
      password: form.value.password,
      totp_code: form.value.totpCode || undefined
    })

    if (response.requires_2fa) {
      requires2FA.value = true
      error.value = '请输入 TOTP 代码'
      loading.value = false
      return
    }

    localStorage.setItem('admin_token', response.token)
    router.push('/')
  } catch (err: any) {
    error.value = err.response?.data?.error || '登录失败'
    if (err.response?.data?.requires_2fa) {
      requires2FA.value = true
    }
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-container {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 100vh;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
}

.login-box {
  background: white;
  padding: 2rem;
  border-radius: 8px;
  box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
  width: 100%;
  max-width: 400px;
}

.login-box h1 {
  text-align: center;
  margin-bottom: 2rem;
  color: #333;
  font-size: 1.5rem;
}

.login-form {
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

.error-message {
  color: #e74c3c;
  font-size: 0.875rem;
  text-align: center;
}

.login-btn {
  padding: 0.75rem;
  background: #667eea;
  color: white;
  border: none;
  border-radius: 4px;
  font-size: 1rem;
  cursor: pointer;
  transition: background 0.2s;
}

.login-btn:hover:not(:disabled) {
  background: #5568d3;
}

.login-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}
</style>

