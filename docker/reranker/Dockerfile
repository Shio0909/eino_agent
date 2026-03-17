# Reranker 微服务 Dockerfile
# 本地 Cross-Encoder 重排序模型服务
FROM python:3.11-slim

WORKDIR /app

# 安装系统依赖
RUN apt-get update && \
    apt-get install -y --no-install-recommends curl && \
    rm -rf /var/lib/apt/lists/*

# 安装 Python 依赖
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# 复制服务代码
COPY main.py .

# 默认环境变量
ENV MODEL_PATH=BAAI/bge-reranker-v2-m3
ENV DEVICE=cpu
ENV PORT=8100

EXPOSE 8100

HEALTHCHECK --interval=30s --timeout=10s --start-period=120s --retries=3 \
    CMD curl -f http://localhost:8100/health || exit 1

CMD ["python", "main.py"]
