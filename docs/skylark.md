# Skylark 引擎与 Knowledge Vault

> **变更说明**：Skylark **渐进式检索**（隐藏工具、`retrieve_knowledge` / `retrieve_capabilities`、会话内 unlock）已移除。Bleve 引擎包名仍为 `pkg/skylark`；对外 API 为 **`api.KnowledgeOptions`** 与两个工具：`memory_search`、`session_search`。

## 当前架构

- **`pkg/skylark`**：Bleve 全文 + 可选向量、`SyncVault` 扫描 Obsidian Vault、历史会话轻量检索。
- **`pkg/api`**：`KnowledgeOptions`、`buildKnowledgeEngine`、`registerKnowledgeTools`；运行时 **不修改** system prompt，**不隐藏**其它工具。

依赖方向：`api` → `skylark`（`skylark` 不依赖 `api`）。

## 配置

```go
Knowledge: &api.KnowledgeOptions{
    Enabled:          true,
    VaultDir:         "", // 默认 <ProjectRoot>/vault 或由宿主解析
    IndexDir:         "", // 默认 <ProjectRoot>/.agents/knowledge-index
    DisableEmbedding: false,
    Embedder:         nil,
},
```

### 环境变量（语义向量，可选）

| 变量 | 说明 |
|------|------|
| `SKYLARK_EMBEDDING_API_KEY` / `OPENAI_API_KEY` | 未设置则仅 Bleve |
| `SKYLARK_EMBEDDING_BASE_URL` / `OPENAI_BASE_URL` | OpenAI 兼容 API |
| `SKYLARK_EMBEDDING_MODEL` | 默认 `text-embedding-3-small` |

## 磁盘布局（`IndexDir`）

```
knowledge-index/
  bleve/          # Bleve index
  corpus.json     # 文档正文
  vectors.json    # 可选向量
```

Vault 源文件在 `VaultDir`（用户可见，Obsidian 可打开）。

## 工具

| 工具 | 数据源 |
|------|--------|
| `memory_search` | Vault `.md` 分块（Bleve + 可选向量） |
| `session_search` | 当前 `message.History`（`SearchHistory`） |

`settings.json` 的 `disallowedTools` 可禁用上述工具名。

## OpenOcta 集成

见 [openocta/docs/knowledge-vault.md](../../openocta/docs/knowledge-vault.md)。
