import { createRouter, createWebHistory } from 'vue-router'
import Login from '../views/Login.vue'
import MailList from '../views/MailList.vue'
import MailView from '../views/MailView.vue'
import Compose from '../views/Compose.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      redirect: '/login'
    },
    {
      path: '/login',
      name: 'Login',
      component: Login
    },
    {
      path: '/mails',
      name: 'MailList',
      component: MailList,
      meta: { requiresAuth: true }
    },
    {
      path: '/mails/:id',
      name: 'MailView',
      component: MailView,
      meta: { requiresAuth: true }
    },
    {
      path: '/compose',
      name: 'Compose',
      component: Compose,
      meta: { requiresAuth: true }
    }
  ]
})

// 路由守卫
router.beforeEach((to, from, next) => {
  const token = localStorage.getItem('token')
  
  // 检查认证要求
  if (to.meta.requiresAuth && !token) {
    next('/login')
    return
  }
  
  next()
})

export default router

