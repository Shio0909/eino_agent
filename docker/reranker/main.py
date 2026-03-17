# Reranker 微服务 - 本地 Cross-Encoder 重排序
# 参考 WeKnora rerank_server_demo.py，使用 FastAPI + Transformers
#
# API: POST /rerank
# Request:  { "query": "...", "documents": ["..."] }
# Response: { "results": [{"index": 0, "relevance_score": 0.95, "document": {"text": "..."}}] }

import os
import logging
import torch
import uvicorn
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from typing import List, Optional
from transformers import AutoModelForSequenceClassification, AutoTokenizer

# ── 配置 ──
MODEL_PATH = os.getenv("MODEL_PATH", "BAAI/bge-reranker-v2-m3")
DEVICE = os.getenv("DEVICE", "cpu")
PORT = int(os.getenv("PORT", "8100"))

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("reranker")

# ── 数据模型 ──
class RerankRequest(BaseModel):
    query: str
    documents: List[str]
    model: Optional[str] = None
    top_n: Optional[int] = None

class DocumentInfo(BaseModel):
    text: str

class RankResult(BaseModel):
    index: int
    relevance_score: float
    document: DocumentInfo

class RerankResponse(BaseModel):
    results: List[RankResult]

# ── 应用 ──
app = FastAPI(title="Eino RAG Reranker", version="1.0.0")

# ── 模型加载 ──
logger.info(f"正在加载 Reranker 模型: {MODEL_PATH}")
logger.info(f"使用设备: {DEVICE}")

try:
    device = torch.device(DEVICE)
    tokenizer = AutoTokenizer.from_pretrained(MODEL_PATH)
    model = AutoModelForSequenceClassification.from_pretrained(MODEL_PATH)
    model.to(device)
    model.eval()
    logger.info("模型加载成功!")
except Exception as e:
    logger.error(f"模型加载失败: {e}")
    logger.warning("服务将以 fallback 模式运行（返回原始顺序）")
    model = None
    tokenizer = None

# ── API 端点 ──
@app.get("/health")
async def health():
    return {
        "status": "ok",
        "model": MODEL_PATH,
        "model_loaded": model is not None,
        "device": DEVICE,
    }

@app.post("/rerank", response_model=RerankResponse)
async def rerank(req: RerankRequest):
    if not req.documents:
        return RerankResponse(results=[])

    top_n = req.top_n or len(req.documents)

    if model is None or tokenizer is None:
        # Fallback: 返回原始顺序，线性衰减分数
        results = []
        for i, doc in enumerate(req.documents[:top_n]):
            results.append(RankResult(
                index=i,
                relevance_score=1.0 - i * 0.05,
                document=DocumentInfo(text=doc),
            ))
        return RerankResponse(results=results)

    try:
        # 构建 query-document pairs
        pairs = [[req.query, doc] for doc in req.documents]

        # Tokenize
        with torch.no_grad():
            inputs = tokenizer(
                pairs,
                padding=True,
                truncation=True,
                max_length=512,
                return_tensors="pt",
            ).to(device)

            # 计算分数
            scores = model(**inputs, return_dict=True).logits.view(-1).float()
            # Sigmoid 归一化
            scores = torch.sigmoid(scores).cpu().tolist()

        # 构建结果并按分数排序
        indexed_scores = list(enumerate(scores))
        indexed_scores.sort(key=lambda x: x[1], reverse=True)

        results = []
        for idx, score in indexed_scores[:top_n]:
            results.append(RankResult(
                index=idx,
                relevance_score=round(score, 6),
                document=DocumentInfo(text=req.documents[idx]),
            ))

        return RerankResponse(results=results)

    except Exception as e:
        logger.error(f"Rerank 失败: {e}")
        raise HTTPException(status_code=500, detail=str(e))

# ── 兼容 Jina Reranker API 格式 ──
@app.post("/v1/rerank", response_model=RerankResponse)
async def rerank_v1(req: RerankRequest):
    """兼容 Jina/Cohere Reranker API 路径"""
    return await rerank(req)

if __name__ == "__main__":
    logger.info(f"启动 Reranker 服务: 0.0.0.0:{PORT}")
    uvicorn.run(app, host="0.0.0.0", port=PORT)
