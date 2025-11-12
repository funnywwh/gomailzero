import { createRouter, createWebHistory } from 'vue-router'
import Login from '../views/Login.vue'
import Init from '../views/Init.vue'
import Dashboard from '../views/Dashboard.vue'
import Users from '../views/Users.vue'
import Domains from '../views/Domains.vue'
import Aliases from '../views/Aliases.vue'
import { api } from '../api'

const router = createRouter({
  history: createWebHistory('/admin'),
  routes: [
    {
      path: '/init',
      name: 'Init',
      component: Init,
      meta: { requiresAuth: false }
    },
    {
      path: '/login',
      name: 'Login',
      component: Login,
      meta: { requiresAuth: false }
    },
    {
      path: '/',
      component: Dashboard,
      meta: { requiresAuth: true },
      children: [
        {
          path: '',
          redirect: '/users'
        },
        {
          path: 'users',
          name: 'Users',
          component: Users
        },
        {
          path: 'domains',
          name: 'Domains',
          component: Domains
        },
        {
          path: 'aliases',
          name: 'Aliases',
          component: Aliases
        }
      ]
    }
  ]
})

router.beforeEach(async (to, from, next) => {
  const token = localStorage.getItem('admin_token')
  
  // 检查认证要求
  if (to.meta.requiresAuth && !token) {
    next('/login')
    return
  }
  
  // 如果访问登录页或根路径，先检查是否需要初始化
  if (to.path === '/login' || to.path === '/') {
    try {
      const response = await api.checkInit()
      if (response.needs_init) {
        // 需要初始化，跳转到初始化页面
        next('/init')
        return
      }
    } catch (err) {
      // 检查失败不影响登录，继续显示登录页面
      console.error('检查初始化状态失败:', err)
    }
  }
  
  // 如果访问初始化页面但系统已初始化，跳转到登录页
  if (to.path === '/init') {
    try {
      const response = await api.checkInit()
      if (!response.needs_init) {
        // 已初始化，跳转到登录页
        next('/login')
        return
      }
    } catch (err) {
      console.error('检查初始化状态失败:', err)
    }
  }
  
  if (to.path === '/login' && token) {
    next('/')
    return
  }
  
  next()
})

export default router

