# GoMailZero 管理后台

GoMailZero 的 Web 管理界面，用于管理用户、域名、别名和配额。

## 功能

- ✅ **用户管理** - 创建、编辑、删除用户，管理用户状态和配额
- ✅ **域名管理** - 管理邮件域名，启用/禁用域名
- ✅ **别名管理** - 创建和管理邮件别名
- ✅ **配额管理** - 查看和设置用户邮箱配额
- ✅ **JWT 认证** - 安全的登录认证，支持 TOTP 双因子认证

## 技术栈

- Vue 3 + TypeScript
- Vite
- Vue Router
- Axios

## 开发

```bash
# 安装依赖
npm install

# 开发模式
npm run dev

# 构建生产版本
npm run build
```

## 访问

管理界面运行在管理 API 服务器上（默认端口 8081），访问地址：

```
http://localhost:8081/admin
```

## 认证

管理界面使用 JWT Token 进行认证。登录后，Token 会存储在浏览器的 `localStorage` 中，后续请求会自动携带 Token。

### 登录

使用已创建的用户邮箱和密码登录。如果用户启用了 TOTP，需要提供 TOTP 代码。

## API 端点

管理界面调用以下 API 端点：

- `POST /api/v1/auth/login` - 登录
- `GET /api/v1/users` - 获取用户列表
- `POST /api/v1/users` - 创建用户
- `PUT /api/v1/users/:email` - 更新用户
- `DELETE /api/v1/users/:email` - 删除用户
- `GET /api/v1/domains` - 获取域名列表
- `POST /api/v1/domains` - 创建域名
- `PUT /api/v1/domains/:name` - 更新域名
- `DELETE /api/v1/domains/:name` - 删除域名
- `GET /api/v1/aliases` - 获取别名列表
- `POST /api/v1/aliases` - 创建别名
- `DELETE /api/v1/aliases/:from` - 删除别名
- `GET /api/v1/users/:email/quota` - 获取用户配额
- `PUT /api/v1/users/:email/quota` - 更新用户配额

## 构建

管理界面会在构建 Go 二进制文件时自动构建并嵌入到二进制文件中。

```bash
make build
```

构建后的静态文件位于 `internal/api/static/` 目录。

