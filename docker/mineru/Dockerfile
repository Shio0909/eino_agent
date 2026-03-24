# MinerU 文档解析微服务 Dockerfile
# 基于 Python 3.11-slim，支持 CPU 推理
# GPU 支持：将基础镜像换为 nvidia/cuda:12.1.0-cudnn8-runtime-ubuntu22.04
# 并在 requirements.txt 中将 magic-pdf[full] 替换为 magic-pdf[full-gpu]
FROM python:3.11-slim

WORKDIR /app

# ── 系统依赖 ──
# libglib/libGL：OpenCV 依赖；curl：healthcheck
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        curl \
        libglib2.0-0 \
        libsm6 \
        libxext6 \
        libxrender1 \
        libgl1-mesa-glx \
        libgomp1 \
    && rm -rf /var/lib/apt/lists/*

# ── Python 依赖 ──
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# ── 服务代码 ──
COPY app.py .

# ── 环境变量默认值 ──
ENV MINERU_DEVICE=cpu
ENV MINERU_METHOD=auto
ENV PORT=8000

# 模型缓存目录（宿主机挂载以避免每次重新下载）
VOLUME ["/root/.mineru"]

EXPOSE 8000

HEALTHCHECK --interval=30s --timeout=15s --start-period=180s --retries=3 \
    CMD curl -f http://localhost:8000/health || exit 1

CMD ["python", "app.py"]
