<template>
  <div class="dashboard">
    <aside class="sidebar">
      <div class="sidebar-header">
        <h2>GoMailZero</h2>
        <p class="user-info">{{ currentUser }}</p>
      </div>
      <nav class="sidebar-nav">
        <router-link to="/users" class="nav-item">
          <span>👥</span> 用户管理
        </router-link>
        <router-link to="/domains" class="nav-item">
          <span>🌐</span> 域名管理
        </router-link>
        <router-link to="/aliases" class="nav-item">
          <span>📧</span> 别名管理
        </router-link>
      </nav>
      <div class="sidebar-footer">
        <button @click="handleLogout" class="logout-btn">退出登录</button>
      </div>
    </aside>
    <main class="main-content">
      <router-view />
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { api, type User } from '../api'

const router = useRouter()
const currentUser = ref('')

onMounted(() => {
  // 从 token 中解析用户信息（简化实现）
  const token = localStorage.getItem('admin_token')
  if (token) {
    // 这里可以从 JWT token 中解析，暂时使用默认值
    currentUser.value = '管理员'
  }
})

const handleLogout = () => {
  localStorage.removeItem('admin_token')
  router.push('/login')
}
</script>

<style scoped>
.dashboard {
  display: flex;
  height: 100vh;
}

.sidebar {
  width: 250px;
  background: #2c3e50;
  color: white;
  display: flex;
  flex-direction: column;
}

.sidebar-header {
  padding: 1.5rem;
  border-bottom: 1px solid #34495e;
}

.sidebar-header h2 {
  font-size: 1.25rem;
  margin-bottom: 0.5rem;
}

.user-info {
  font-size: 0.875rem;
  color: #bdc3c7;
}

.sidebar-nav {
  flex: 1;
  padding: 1rem 0;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.75rem 1.5rem;
  color: #bdc3c7;
  text-decoration: none;
  transition: background 0.2s;
}

.nav-item:hover {
  background: #34495e;
}

.nav-item.router-link-active {
  background: #34495e;
  color: white;
  border-left: 3px solid #667eea;
}

.sidebar-footer {
  padding: 1rem;
  border-top: 1px solid #34495e;
}

.logout-btn {
  width: 100%;
  padding: 0.75rem;
  background: #e74c3c;
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  transition: background 0.2s;
}

.logout-btn:hover {
  background: #c0392b;
}

.main-content {
  flex: 1;
  overflow-y: auto;
  background: #f5f5f5;
}
</style>

