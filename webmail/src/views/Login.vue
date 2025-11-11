<template>
  <div class="login-container">
    <div class="login-box">
      <h1>GoMailZero</h1>
      <div v-if="checkingInit" class="checking">检查系统状态...</div>
      <form v-else @submit.prevent="handleLogin">
        <div class="form-group">
          <label for="email">邮箱</label>
          <input
            id="email"
            v-model="email"
            type="email"
            required
            placeholder="your@email.com"
          />
        </div>
        <div class="form-group">
          <label for="password">密码</label>
          <input
            id="password"
            v-model="password"
            type="password"
            required
            placeholder="••••••••"
          />
        </div>
        <div v-if="requiresTOTP" class="form-group">
          <label for="totp">TOTP 代码</label>
          <input
            id="totp"
            v-model="totpCode"
            type="text"
            required
            placeholder="000000"
            maxlength="6"
          />
        </div>
        <div v-if="error" class="error">{{ error }}</div>
        <button type="submit" :disabled="loading">
          {{ loading ? '登录中...' : '登录' }}
        </button>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api'

const router = useRouter()

const email = ref('')
const password = ref('')
const totpCode = ref('')
const requiresTOTP = ref(false)
const loading = ref(false)
const error = ref('')
const checkingInit = ref(true)

const checkInit = async () => {
  try {
    const response = await api.checkInit()
    if (response.needs_init) {
      // 需要初始化，跳转到初始化页面
      router.push('/init')
    }
  } catch (err) {
    console.error('检查初始化状态失败:', err)
    // 检查失败不影响登录，继续显示登录页面
  } finally {
    checkingInit.value = false
  }
}

const handleLogin = async () => {
  loading.value = true
  error.value = ''

  try {
    const response = await api.login({
      email: email.value,
      password: password.value,
      totp_code: requiresTOTP.value ? totpCode.value : undefined
    })

    if (response.requires_2fa) {
      requiresTOTP.value = true
      loading.value = false
      return
    }

    if (response.token) {
      localStorage.setItem('token', response.token)
      router.push('/mails')
    }
  } catch (err: any) {
    error.value = err.response?.data?.error || '登录失败'
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  checkInit()
})
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

h1 {
  text-align: center;
  margin-bottom: 2rem;
  color: #333;
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
}

input:focus {
  outline: none;
  border-color: #667eea;
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
}

.checking {
  text-align: center;
  color: #666;
  padding: 2rem;
}
</style>

