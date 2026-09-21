# RAG 迭代与评测工作流

## Chunk 人工迭代

1. 在 Knowledge 页面打开文档并查看 Chunk。
2. 编辑内容并保存。服务会把旧内容写入 `chunk_revisions`。
3. 运行“更新索引”，只对当前 Chunk 重新生成 embedding，不会重新解析原始文档。
4. 在“历史”中查看旧版本，必要时回滚。
5. 对修改前后的同一评测集运行回归，比较 Recall、MRR、nDCG、Faithfulness 和延迟。

编辑接口：

```text
PATCH /api/v1/knowledge-bases/:kb/documents/:doc/chunks/:chunk
GET   /api/v1/knowledge-bases/:kb/documents/:doc/chunks/:chunk/revisions
POST  /api/v1/knowledge-bases/:kb/documents/:doc/chunks/:chunk/reindex
POST  /api/v1/knowledge-bases/:kb/documents/:doc/chunks/:chunk/revisions/:revision/restore
```
