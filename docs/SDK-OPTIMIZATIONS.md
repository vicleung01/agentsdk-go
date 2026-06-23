# agentsdk-go 优化能力使用指南（调用方）

更新时间：2026-04-17

本文面向 **agentsdk-go SDK 调用者**，汇总仓库内已实现的优化能力与推荐配置，帮助你在不同产品形态（CLI / HTTP / 内嵌服务）中稳定获得：

- **Knowledge Vault 检索**：Obsidian 兼容 Vault + Bleve（可选向量），`memory_search` / `session_search`
- **MCP 工具描述/Schema 更省 token、更稳定**
- **工具输出不再淹没上下文（落盘 + 引用 + 摘要）**
- **结构化反思记录（便于上层做重试/降级/落盘）**

> 说明：仓库 v2 的“ground truth”在 `docs/refactor/PRD.md` 与 `docs/refactor/ARCHITECTURE-v2.md`。本文只解释“如何用”，不展开全部内部实现细节。

---

## 1. 一句话架构：四层记忆（L0-L3）

- **L0 瞬时工作记忆**：本次 run 的中间态（`middleware.State.Values` 等），不落盘。
- **L1 会话记忆**：`message.History`（单 session 多轮对话）；`session_search` 检索当前会话。
- **L3 知识库（Knowledge）**：Obsidian Vault `.md` → Bleve 索引；`memory_search` 检索。

> 注：旧版 Skylark progressive（隐藏工具、`retrieve_*`、L2 JSONL `project_memory.jsonl`）已移除。长期笔记请放入 Vault；Evolution 层仍提供 curated 记忆工具。

你通常只需要：**启用 Knowledge + 保持默认压缩阈值**，即可获得稳定检索体验。

---

## 2. 快速开始（推荐默认）

### 2.1 推荐目录结构

项目内放置：

```text
vault/                 # Obsidian 兼容笔记（用户可见）
knowledge-index/       # Bleve 派生索引（gitignore）
.agents/
├── settings.json
├── rules/
└── skills/
```

并确保 `.gitignore` 已忽略 `knowledge-index/` 等索引产物。

### 2.2 Go 侧最小配置（示例）

```go
rt, err := api.New(api.Options{
  ProjectRoot: "/path/to/project",
  // SystemPrompt / Model / Tools / MCPServers 等按你的应用注入

  Knowledge: &api.KnowledgeOptions{
    Enabled:  true,
    VaultDir: "/path/to/project/vault",
    IndexDir: "/path/to/state/knowledge-index",
  },

  // 工具输出压缩（避免 history/token 爆炸）
  ToolOutputInlineMaxRunes:  4000,
  ToolOutputSnippetMaxRunes: 900,

  // 结构化反思（默认启用；显式写出便于阅读）
  ReflectionEnabled: ptr(true),
})
```

> 提示：若你不设置这些字段，SDK 会提供默认值；上面的代码只是把常用 knobs 显式化，便于团队讨论/调参。

---

## 3. Knowledge Vault 检索

### 3.1 工具

- **`memory_search`**：检索 Vault 中 Markdown 分块（Bleve + 可选向量）。
- **`session_search`**：检索当前会话 `History`（复问场景可主动调用）。

Agent 运行时 **不隐藏** 其它工具；Knowledge 不再修改 system prompt。

### 3.2 配置

见 [`docs/skylark.md`](./skylark.md)（Knowledge API）与 OpenOcta [`knowledge-vault.md`](../../openocta/docs/knowledge-vault.md)。

---

## 4. Evolution 记忆（可选）

L4 Evolution 的 `memory` 工具与 Knowledge Vault **独立**，用于 agent 维护 `.agents/evolution/` 下 curated 文件。见 `pkg/api/evolution*.go`。

---

## 5. MCP 工具优化（省 token + 更稳定）

SDK 针对 MCP tool 的常见痛点做了两类优化：

- **描述与 schema 压缩（tool registry 层）**：裁剪冗余字段、缩短 description，降低提示词体积。
- **刷新稳定性**：对 `ToolListChanged` 做 debounce / singleflight / backoff，避免 refresh 风暴。

调用方通常只需要按业务启用 MCP servers；压缩策略与刷新稳定性对你是“透明收益”。

---

## 6. 工具输出压缩（落盘 + 引用 + 摘要）

### 6.1 解决什么问题

很多工具（bash、搜索、抓取）会产出长输出；把它们原样塞进 history 会：

- 迅速消耗 token
- 干扰模型注意力（大量无关文本）
- 让 compaction 变得困难（I/O 淹没上下文）

### 6.2 SDK 的策略（已实现）

当某次工具输出超过阈值（`ToolOutputInlineMaxRunes`）：

- **全文落盘**到本机 spool（示例：`/tmp/agentsdk/tool-output/...`）
- history 中只保留：
  - **引用指针**（保存路径）
  - **高信息密度摘要**

摘要规则（优先级）：

- **JSON**：输出 object keys / array len
- **日志/多行文本**：输出 head + tail
- 其他：裁剪到 snippet 上限

### 6.3 调参建议（api.Options）

- **`ToolOutputInlineMaxRunes`**：越小越“省 token”，但可能让模型更频繁依赖引用取回。
- **`ToolOutputSnippetMaxRunes`**：越大越“可读”，但会增加 history 成本。

---

## 7. 结构化反思（Reflection）

SDK 默认启用结构化反思中间件，用于把“失败”变成机器可处理的记录，写入 `middleware.State.Values`（不会阻塞主流程）。

覆盖场景包括：

- 工具调用失败：timeout / validation / safety_denied / MCP 问题等
- 模型因 token 限制等原因提前停止（可用于上层降级/重试）

调用方建议：

- 在你自己的日志/观测系统里，把 `reflection.records`（若存在）打到结构化日志，方便分析“为什么失败、下一步该怎么做”。

---

## 8. 安全与兼容性注意点

- **bash 工具更严格**：默认不允许 shell 元字符/多行命令（除非显式允许）。这属于安全收益，但可能影响你之前依赖的复杂命令拼接。
- **OpenAI provider 更严格**：`api key` 为空会直接报错（避免测试/环境变量混乱）。
- **Knowledge 索引产物**：应保持在 gitignore 内；可共享笔记应放 `vault/`。

---

## 9. 常见问题（FAQ）

### 9.1 为什么 memory_search 没有命中？

- Vault 路径是否正确（OpenOcta 默认 `~/.openocta/vault` 或 workspace 下 `vault/`）
- 是否在 Agent 启动后编辑笔记（需重启或下次启动 rebuild）
- 查询是否与笔记内容相关（Bleve 全文 + 可选向量）

### 9.2 session_search 为什么为空？

- 当前 run 的 `History` 是否已加载（跨进程续聊需配置 session history loader）

### 9.3 工具输出落盘路径是否可控？

当前默认落盘在临时目录下，适合本机/短生命周期进程。若你在容器或多机环境运行，建议后续按 P2.3-C 路线把策略下沉到 `tool.OutputPersister` 并支持自定义 URI（例如对象存储、可观测平台、内部文件服务）。

---

## 10. 后续路线（可选）

如果你希望进一步“统一输出压缩策略”，建议按 `docs/OPTIMIZATION-PLAN.md` 的 **P2.3-C** 推进：

- 下沉到 `tool.Executor` 的 `OutputPersister`（让所有运行时路径共享同一策略）
- 把 API 层压缩逻辑变为兼容层，逐步移除重复落盘

