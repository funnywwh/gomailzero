# GoMailZero WebMail 前端

Vue3 + Vite + TypeScript 构建的现代化 WebMail 前端。

## 开发

```bash
# 安装依赖
npm install

# 启动开发服务器
npm run dev

# 构建生产版本（输出到 internal/web/static）
npm run build

# 预览生产构建
npm run preview
```

## 功能

- ✅ 登录页面（支持 TOTP 2FA）
- ✅ 邮件列表页面（文件夹导航、未读标记）
- ✅ 邮件阅读页面（自动标记已读）
- ✅ 邮件编写页面（支持草稿保存）
- ✅ 文件夹管理（动态加载、中文显示）
- ✅ 搜索功能（按主题、发件人、收件人）
- ✅ 邮件标记（已读/未读）
- ✅ 回复和转发功能
- 🚧 PGP 加密/签名
- 🚧 邮件附件支持

## 技术栈

- Vue 3 (Composition API)
- Vue Router 4
- Pinia (状态管理)
- Axios (HTTP 客户端)
- TypeScript
- Vite (构建工具)

