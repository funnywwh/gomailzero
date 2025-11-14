# IMAP 标准符合性检查报告

基于 RFC 3501 (IMAP4rev1) 标准，详细检查当前实现中的问题。

## 🔴 严重问题（违反 RFC 3501 标准）

### 1. UID 实现不符合标准

**问题位置：**
- `internal/imapd/backend.go:950, 981, 1179`

**问题描述：**
```go
// 目前我们使用序列号作为 UID（TODO: 使用实际的 UID），所以两种情况都使用 seqNum
checkNum := seqNum
msg.Uid = seqNum // TODO: 使用实际的 UID
```

**RFC 3501 要求：**
- UID 必须是唯一的、持久的、单调递增的标识符
- UID 在邮箱的整个生命周期内必须保持不变
- UID 不能等于序列号（除非序列号恰好等于 UID）
- UID FETCH 和普通 FETCH 必须使用不同的数值空间

**当前问题：**
- ❌ UID 等于序列号，违反了 UID 的唯一性和持久性要求
- ❌ 当邮件被删除或移动时，序列号会改变，但 UID 不应该改变
- ❌ UID FETCH 和普通 FETCH 使用相同的数值，导致客户端混淆

**影响：**
- 客户端无法正确跟踪邮件（UID 会随序列号变化）
- 邮件移动/删除后，UID 映射失效
- 不符合 IMAP 客户端对 UID 稳定性的预期

---

### 2. UIDVALIDITY 固定为 1

**问题位置：**
- `internal/imapd/backend.go:875`

**问题描述：**
```go
case imap.StatusUidValidity:
    status.UidValidity = 1
```

**RFC 3501 要求：**
- UIDVALIDITY 必须在邮箱被清空或重新创建时改变
- UIDVALIDITY 应该是一个大的随机数或时间戳
- 如果 UIDVALIDITY 改变，客户端必须重新同步所有邮件

**当前问题：**
- ❌ UIDVALIDITY 固定为 1，永远不会改变
- ❌ 如果邮箱被清空重建，UIDVALIDITY 应该改变但实际没有

**影响：**
- 客户端可能使用过期的 UID 缓存
- 无法正确检测邮箱是否被重建

---

### 3. 序列号计算错误（邮件删除后不更新）

**问题位置：**
- `internal/imapd/backend.go:942-945`

**问题描述：**
```go
for i, mail := range m.mails {
    seqNum := uint32(i + 1)
```

**RFC 3501 要求：**
- 序列号必须从 1 开始，连续递增
- 删除邮件后，后续邮件的序列号必须重新计算
- 序列号必须在 SELECT 后保持稳定（直到 EXPUNGE）

**当前问题：**
- ⚠️ 如果 `m.mails` 列表包含已删除的邮件（标记为 \Deleted），序列号会跳过
- ⚠️ EXPUNGE 后序列号会改变，但客户端可能已经缓存了旧的序列号

**影响：**
- 客户端 FETCH 可能使用错误的序列号
- 邮件删除后序列号不连续

---

### 4. SearchMessages 不支持 UID SEARCH

**问题位置：**
- `internal/imapd/backend.go:1508-1601`

**问题描述：**
```go
func (m *Mailbox) SearchMessages(uid bool, criteria *imap.SearchCriteria) ([]uint32, error) {
    // ...
    seqNum := uint32(i + 1)
    // ...
    if matched {
        results = append(results, seqNum)  // 总是返回序列号，忽略 uid 参数
    }
}
```

**RFC 3501 要求：**
- `uid=false` 时，SEARCH 返回序列号
- `uid=true` 时，UID SEARCH 必须返回 UID 值
- 两种模式必须返回不同的数值

**当前问题：**
- ❌ 无论 `uid` 参数如何，都返回序列号
- ❌ UID SEARCH 和普通 SEARCH 返回相同的结果

**影响：**
- 客户端使用 UID SEARCH 时得到错误的 UID 值
- 无法正确使用 UID 进行邮件操作

---

### 5. UpdateMessagesFlags 不支持 UID STORE

**问题位置：**
- `internal/imapd/backend.go:1811-1876`

**问题描述：**
```go
func (m *Mailbox) UpdateMessagesFlags(uid bool, seqSet *imap.SeqSet, op imap.FlagsOp, flags []string) error {
    // ...
    seqNum := uint32(i + 1)
    if seqSet != nil && !seqSet.Contains(seqNum) {  // 总是使用序列号匹配
        continue
    }
}
```

**RFC 3501 要求：**
- `uid=false` 时，STORE 使用序列号匹配
- `uid=true` 时，UID STORE 必须使用 UID 匹配
- 必须根据 `uid` 参数选择正确的匹配方式

**当前问题：**
- ❌ 无论 `uid` 参数如何，都使用序列号匹配
- ❌ UID STORE 无法正确工作

**影响：**
- 客户端使用 UID STORE 时无法正确更新标志
- 邮件操作失败

---

### 6. CopyMessages 不支持 UID COPY

**问题位置：**
- `internal/imapd/backend.go:1879-1923`

**问题描述：**
```go
func (m *Mailbox) CopyMessages(uid bool, seqSet *imap.SeqSet, dest string) error {
    // ...
    seqNum := uint32(i + 1)
    if seqSet != nil && !seqSet.Contains(seqNum) {  // 总是使用序列号匹配
        continue
    }
}
```

**RFC 3501 要求：**
- `uid=false` 时，COPY 使用序列号匹配
- `uid=true` 时，UID COPY 必须使用 UID 匹配

**当前问题：**
- ❌ 无论 `uid` 参数如何，都使用序列号匹配
- ❌ UID COPY 无法正确工作

---

### 7. \Recent 标志处理不符合标准

**问题位置：**
- `internal/imapd/backend.go:1227-1274, 1397-1446`

**问题描述：**
```go
// 自动设置 \Seen 标志
// 移除 \Recent 标志（如果存在）
if hasRecent {
    flagMap := make(map[string]bool)
    for _, f := range newFlags {
        if f != imap.RecentFlag {
            flagMap[f] = true
        }
    }
    newFlags = make([]string, 0, len(flagMap))
    for f := range flagMap {
        newFlags = append(newFlags, f)
    }
}
```

**RFC 3501 要求：**
- \Recent 标志只能由服务器设置，客户端不能设置或清除
- \Recent 标志在 SELECT 后自动清除（除了新邮件）
- 读取邮件（非 PEEK）后，\Recent 应该被清除，但不应在设置 \Seen 时自动清除

**当前问题：**
- ⚠️ 在设置 \Seen 时自动移除 \Recent，这不符合标准行为
- ⚠️ \Recent 应该在 SELECT 时统一处理，而不是在每次 FETCH 时处理

---

### 8. BODY.PEEK 不应该设置 \Seen 标志

**问题位置：**
- `internal/imapd/backend.go:1388-1428`

**问题描述：**
```go
// 根据 IMAP 规范，如果客户端使用 FETCH（不是 PEEK）获取邮件体，自动设置 \Seen 标志
// 为了兼容 Foxmail 等客户端，即使使用 PEEK，也设置 \Seen 标志
// 如果邮件还没有 \Seen 标志，设置它（即使使用 PEEK，也设置以兼容 Foxmail）
if !hasSeen {
    // 自动设置 \Seen 标志
}
```

**RFC 3501 要求：**
- BODY.PEEK 明确表示"不设置 \Seen 标志"
- 只有 BODY（非 PEEK）才应该设置 \Seen 标志
- 服务器必须严格遵守 PEEK 语义

**当前问题：**
- ❌ BODY.PEEK 也设置 \Seen 标志，违反了 RFC 3501
- ❌ 这是为了"兼容 Foxmail"的变通，但不符合标准

**影响：**
- 客户端使用 PEEK 时，邮件被意外标记为已读
- 不符合 IMAP 标准行为

---

### 9. Envelope 字段不完整

**问题位置：**
- `internal/imapd/backend.go:1050-1056`

**问题描述：**
```go
msg.Envelope = &imap.Envelope{
    Subject: mail.Subject,
    From:    fromAddrs,
    To:      toAddrs,
    Date:    date,
    // 缺少：Cc, Bcc, Reply-To, In-Reply-To, Message-ID, References, Sender
}
```

**RFC 3501 要求：**
- Envelope 必须包含所有标准字段（如果邮件中存在）
- 至少应该包含：From, To, Cc, Bcc, Reply-To, In-Reply-To, Message-ID, References, Sender, Subject, Date

**当前问题：**
- ⚠️ Envelope 缺少多个标准字段
- ⚠️ 客户端可能无法正确显示邮件信息

---

### 10. BodyStructure 实现过于简化

**问题位置：**
- `internal/imapd/backend.go:1187-1202`

**问题描述：**
```go
msg.BodyStructure = &imap.BodyStructure{
    MIMEType:    "text",
    MIMESubType: "plain",
    Size:        size,
    // 缺少：Parameters, Disposition, Language, Location, Extension
}
```

**RFC 3501 要求：**
- BodyStructure 应该完整解析 MIME 结构
- 必须支持 multipart、message/rfc822 等复杂类型
- 应该包含 Content-Type 参数、Content-Disposition 等

**当前问题：**
- ❌ 所有邮件都被视为 text/plain
- ❌ 不支持 multipart 邮件
- ❌ 不支持附件

**影响：**
- 客户端无法正确显示多部分邮件
- 附件无法识别
- HTML 邮件无法正确显示

---

### 11. 自动添加 Envelope（即使未请求）

**问题位置：**
- `internal/imapd/backend.go:921-940, 1050-1056`

**问题描述：**
```go
// 检查是否请求了 BODY 但没有请求 Envelope，如果是，也添加 Envelope
if hasBodyRequest && !hasEnvelopeRequest {
    items = append(items, imap.FetchEnvelope)
}
// 预先填充 Envelope（即使客户端没有请求）
msg.Envelope = &imap.Envelope{...}
```

**RFC 3501 要求：**
- 服务器必须只返回客户端请求的字段
- 不能自动添加未请求的字段
- 客户端明确请求的字段顺序必须保持

**当前问题：**
- ⚠️ 自动添加 Envelope，违反了"只返回请求的字段"原则
- ⚠️ 可能影响响应大小和性能

**注意：** 虽然这是为了兼容性，但不符合严格的标准

---

### 12. 自动设置 \Seen 标志（兼容性变通）

**问题位置：**
- `internal/imapd/backend.go:1112-1138, 1220-1256, 1388-1428`

**问题描述：**
```go
// 为了兼容 Foxmail，当客户端请求 FLAGS 时，也自动设置 \Seen 标志
// 如果邮件没有 \Seen 标志，且没有 \Recent 标志，自动设置 \Seen 标志（兼容 Foxmail）
// 即使使用 PEEK，也设置以兼容 Foxmail
```

**RFC 3501 要求：**
- \Seen 标志只能由客户端显式设置，或通过 FETCH（非 PEEK）自动设置
- 服务器不应该自动设置 \Seen 标志（除非符合标准情况）

**当前问题：**
- ❌ 在请求 FLAGS 时自动设置 \Seen（不符合标准）
- ❌ 在 BODY.PEEK 时设置 \Seen（违反 PEEK 语义）

---

### 13. Status 响应中 UIDNEXT 计算错误

**问题位置：**
- `internal/imapd/backend.go:860-873`

**问题描述：**
```go
case imap.StatusUidNext:
    if len(m.mails)+1 <= int(^uint32(0)) {
        status.UidNext = uint32(len(m.mails) + 1)
    }
```

**RFC 3501 要求：**
- UIDNEXT 应该是下一个要分配的 UID
- 如果当前最大 UID 是 N，UIDNEXT 应该是 N+1
- 不能简单地使用邮件数量+1

**当前问题：**
- ❌ 使用邮件数量+1，而不是实际的最大 UID+1
- ❌ 如果邮件被删除，UIDNEXT 会变小（不符合 UID 单调递增）

---

### 14. SearchMessages 不支持 UID 参数

**问题位置：**
- `internal/imapd/backend.go:1508`

**问题描述：**
```go
func (m *Mailbox) SearchMessages(uid bool, criteria *imap.SearchCriteria) ([]uint32, error) {
    // ...
    results = append(results, seqNum)  // 总是返回序列号
}
```

**问题：**
- ❌ `uid` 参数被忽略，总是返回序列号
- ❌ UID SEARCH 无法工作

---

### 15. 邮件排序不稳定

**问题位置：**
- `internal/imapd/backend.go:520-530`

**问题描述：**
```go
// 重新排序邮件（按接收时间降序）
if len(mails) > 0 {
    sort.Slice(mails, func(i, j int) bool {
        return mails[i].ReceivedAt.After(mails[j].ReceivedAt)
    })
}
```

**RFC 3501 要求：**
- 邮件顺序应该在 SELECT 后保持稳定
- 序列号应该反映稳定的顺序
- 不应该在每次 GETMAILBOX 时重新排序

**当前问题：**
- ⚠️ 每次 GETMAILBOX 都重新排序，导致序列号不稳定
- ⚠️ 客户端缓存的序列号可能失效

---

## 🟡 中等问题（部分不符合或实现不完整）

### 16. LIST/LSUB 响应格式

**问题位置：**
- `internal/imapd/backend.go:69-151`

**问题：**
- ⚠️ 没有正确实现订阅状态（subscribed 参数被忽略）
- ⚠️ 邮箱属性可能不完整

---

### 17. SELECT/EXAMINE 响应不完整

**问题位置：**
- `internal/imapd/backend.go:786-880` (Status 方法)

**问题：**
- ⚠️ SELECT 响应应该包含完整的邮箱状态
- ⚠️ 可能缺少某些必需的状态项

---

### 18. SEARCH 命令实现不完整

**问题位置：**
- `internal/imapd/backend.go:1507-1601`

**问题：**
- ⚠️ 只支持部分搜索条件（WithFlags, WithoutFlags, Header, Body, SeqNum）
- ⚠️ 缺少：BEFORE, SINCE, LARGER, SMALLER, ON, SENTBEFORE, SENTSINCE 等
- ⚠️ 搜索是区分大小写的（应该不区分）

---

### 19. COPY 命令实现不完整

**问题位置：**
- `internal/imapd/backend.go:1879-1923`

**问题：**
- ⚠️ 复制邮件时，新邮件的 UID 生成方式不正确
- ⚠️ 应该使用目标邮箱的 UIDNEXT，而不是简单的计数

---

### 20. EXPUNGE 响应格式

**问题位置：**
- `internal/imapd/backend.go:1926-1964`

**问题：**
- ⚠️ EXPUNGE 应该返回被删除邮件的序列号列表
- ⚠️ 当前实现可能没有正确发送 EXPUNGE 响应

---

### 21. 地址解析过于简化

**问题位置：**
- `internal/imapd/backend.go:1967-1980, 1773-1792`

**问题：**
- ⚠️ `parseEmailAddress` 和 `parseAddressList` 实现过于简单
- ⚠️ 不支持复杂的地址格式（如带引号的显示名称）
- ⚠️ 不支持地址组（group syntax）

---

### 22. 字符串搜索区分大小写

**问题位置：**
- `internal/imapd/backend.go:1604-1615`

**问题：**
```go
// 使用简单的字符串包含检查（区分大小写）
func contains(s, substr string) bool {
    // ...
}
```

**RFC 3501 要求：**
- SEARCH 命令的字符串匹配应该不区分大小写（除非明确指定）

**问题：**
- ⚠️ 当前实现区分大小写

---

## 🟢 轻微问题（实现细节）

### 23. 错误消息格式

**问题：**
- ⚠️ 某些错误消息可能不符合 IMAP 响应格式
- ⚠️ 应该使用标准的 IMAP 错误代码

---

### 24. 日志过多

**问题：**
- ⚠️ 生产环境可能产生过多调试日志
- ⚠️ 应该根据日志级别控制输出

---

## 📋 总结

### 严重问题（必须修复）：
1. ✅ UID 实现（使用序列号代替 UID）
2. ✅ UIDVALIDITY 固定值
3. ✅ UID SEARCH/STORE/COPY 不支持
4. ✅ BODY.PEEK 设置 \Seen 标志
5. ✅ BodyStructure 实现不完整
6. ✅ Envelope 字段不完整

### 中等问题（建议修复）：
7. ⚠️ \Recent 标志处理
8. ⚠️ 邮件排序稳定性
9. ⚠️ SEARCH 命令不完整
10. ⚠️ 地址解析简化

### 优先级建议：
1. **最高优先级**：修复 UID 实现（影响所有客户端）
2. **高优先级**：修复 UID SEARCH/STORE/COPY（影响 UID 操作）
3. **中优先级**：修复 BodyStructure 和 Envelope（影响邮件显示）
4. **低优先级**：完善 SEARCH 和地址解析

