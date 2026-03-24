# MinerU 文档解析微服务
#
# 将 MinerU (magic-pdf) 封装为 FastAPI HTTP 服务，供 Go 主服务调用。
#
# API:
#   POST /parse/file   - 解析上传的文件（multipart/form-data）
#   GET  /health       - 健康检查
#
# 环境变量:
#   MINERU_DEVICE      - 推理设备: cpu / cuda  (默认: cpu)
#   MINERU_METHOD      - 解析方法: auto / ocr / txt  (默认: auto)
#   PORT               - 监听端口 (默认: 8000)

import io
import logging
import os
import re
import subprocess
import tempfile
import unicodedata
from pathlib import Path
from typing import List

import uvicorn
from fastapi import FastAPI, File, Form, HTTPException, UploadFile
from pydantic import BaseModel

# ── 配置 ──
DEVICE  = os.getenv("MINERU_DEVICE",  "cpu")
METHOD  = os.getenv("MINERU_METHOD",  "auto")   # auto | ocr | txt
PORT    = int(os.getenv("PORT", "8000"))

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("mineru-service")

# ── 数据模型 ──

class ParsedChunk(BaseModel):
    content: str
    seq: int

class ParseResponse(BaseModel):
    chunks: List[ParsedChunk]

# ── MinerU 延迟初始化 ──
# 延迟导入，避免未安装时直接崩溃；启动时才真正加载模型

_mineru_ready: bool = False

def _try_init_mineru() -> bool:
    global _mineru_ready
    if _mineru_ready:
        return True
    try:
        import magic_pdf.model as model_config  # noqa: F401
        model_config.model_mode = "full"
        _mineru_ready = True
        logger.info("MinerU 初始化成功")
        return True
    except Exception as exc:
        logger.warning(f"MinerU 初始化失败，将降级为纯文本提取: {exc}")
        return False

# ── FastAPI 应用 ──

app = FastAPI(title="MinerU Document Parser", version="1.0.0")

@app.on_event("startup")
async def startup_event():
    _try_init_mineru()

@app.get("/health")
def health():
    return {"status": "ok", "mineru_ready": _mineru_ready}

# ── 核心解析接口 ──

@app.post("/parse/file", response_model=ParseResponse)
async def parse_file(
    file: UploadFile = File(...),
    chunk_size: int   = Form(500),
    chunk_overlap: int = Form(50),
):
    """接收文件字节，通过 MinerU 解析为文本 chunks。"""
    raw = await file.read()
    if not raw:
        raise HTTPException(status_code=400, detail="上传文件为空")

    filename = file.filename or "document.pdf"
    ext = Path(filename).suffix.lower()

    try:
        text = await _extract_text(raw, filename, ext)
    except Exception as exc:
        logger.exception(f"解析失败: {filename}")
        raise HTTPException(status_code=500, detail=f"解析失败: {exc}") from exc

    chunks = _chunk_text(text, chunk_size, chunk_overlap)
    return ParseResponse(
        chunks=[ParsedChunk(content=c, seq=i) for i, c in enumerate(chunks)]
    )

# ── 文本提取 ──

async def _extract_text(raw: bytes, filename: str, ext: str) -> str:
    """根据文件类型选择提取策略。"""
    # 纯文本类型直接解码
    if ext in (".txt", ".md", ".rst", ".log", ".csv", ".json"):
        return raw.decode("utf-8", errors="replace")

    # HTML 简单提取标签文本
    if ext in (".html", ".htm"):
        return _strip_html(raw.decode("utf-8", errors="replace"))

    # PDF 及其他格式：优先 MinerU，失败则 CLI
    if ext == ".pdf":
        if _try_init_mineru():
            return _mineru_parse_pdf(raw)
        return _fallback_cli_parse(raw, filename)

    # 其他格式（DOCX、PPT 等）：尝试 MinerU CLI
    return _fallback_cli_parse(raw, filename)


def _mineru_parse_pdf(raw: bytes) -> str:
    """通过 MinerU Python API 解析 PDF，返回纯文本。"""
    import magic_pdf.model as model_config
    from magic_pdf.config.make_content_config import DropMode, MakeMode
    from magic_pdf.data.data_reader_writer import DataWriter
    from magic_pdf.pipe.UNIPipeline import UNIPipeline

    class _MemWriter(DataWriter):
        """将 MinerU 输出写入内存，不落磁盘。"""
        def __init__(self):
            self.data: dict[str, bytes] = {}

        def write(self, path: str, data: bytes) -> None:
            self.data[path] = data

        def write_string(self, path: str, data: str) -> None:
            self.data[path] = data.encode("utf-8")

    writer = _MemWriter()
    pipe = UNIPipeline(
        pdf_bytes=raw,
        data_writer=writer,
        is_debug=False,
        image_writer=writer,
    )
    pipe.pipe_classify()
    if METHOD == "ocr":
        pipe.pipe_parse()
    elif METHOD == "txt":
        pipe.pipe_parse()
    else:
        pipe.pipe_parse()
    pipe.pipe_mk_markdown(writer, drop_mode=DropMode.NONE, md_make_mode=MakeMode.MM_MD)

    # 读取生成的 Markdown 内容
    md_bytes: bytes = b""
    for key, val in writer.data.items():
        if key.endswith(".md"):
            md_bytes = val
            break
    if not md_bytes:
        # 没有 Markdown，尝试拼接所有文本数据
        md_bytes = b"\n".join(
            v for k, v in writer.data.items()
            if isinstance(v, bytes) and k.endswith((".txt", ".md"))
        )

    text = md_bytes.decode("utf-8", errors="replace")
    return _clean_markdown(text)


def _fallback_cli_parse(raw: bytes, filename: str) -> str:
    """通过 mineru CLI 解析文件，兜底方案。"""
    with tempfile.TemporaryDirectory() as tmpdir:
        in_path  = os.path.join(tmpdir, filename)
        out_dir  = os.path.join(tmpdir, "out")
        os.makedirs(out_dir, exist_ok=True)

        with open(in_path, "wb") as fh:
            fh.write(raw)

        result = subprocess.run(
            ["mineru", "-p", in_path, "-o", out_dir, "-m", METHOD],
            capture_output=True,
            timeout=300,
        )
        if result.returncode != 0:
            stderr = result.stderr.decode("utf-8", errors="replace")
            raise RuntimeError(f"mineru CLI 返回非零退出码: {stderr[:500]}")

        # 查找生成的 Markdown 文件
        md_files = list(Path(out_dir).rglob("*.md"))
        if not md_files:
            raise RuntimeError("mineru CLI 未生成任何 Markdown 文件")

        texts = []
        for md in sorted(md_files):
            texts.append(md.read_text("utf-8", errors="replace"))
        return _clean_markdown("\n\n".join(texts))


# ── 工具函数 ──

def _strip_html(html: str) -> str:
    """去掉 HTML 标签，返回纯文本。"""
    text = re.sub(r"<script[^>]*>.*?</script>", "", html, flags=re.DOTALL | re.IGNORECASE)
    text = re.sub(r"<style[^>]*>.*?</style>",  "", text,  flags=re.DOTALL | re.IGNORECASE)
    text = re.sub(r"<[^>]+>", " ", text)
    text = re.sub(r"&[a-zA-Z]+;",  " ", text)
    text = re.sub(r"&#?\w+;",      " ", text)
    return re.sub(r"\s{2,}", "\n", text).strip()


def _clean_markdown(text: str) -> str:
    """去除 Markdown 图片语法和多余空行，保留纯文本内容。"""
    text = re.sub(r"!\[.*?\]\(.*?\)", "", text)   # 图片
    text = re.sub(r"\[([^\]]+)\]\([^)]+\)", r"\1", text)  # 链接 → 锚文本
    text = re.sub(r"\n{3,}", "\n\n", text)
    return text.strip()


def _chunk_text(text: str, chunk_size: int, chunk_overlap: int) -> List[str]:
    """按 Unicode 字符（rune）进行固定大小分块，与 Go 实现保持一致。"""
    text = text.strip()
    if not text:
        return []

    runes = list(text)          # Python str 是 Unicode 码点列表
    if chunk_size <= 0:
        chunk_size = 500
    if chunk_overlap < 0:
        chunk_overlap = 0
    if chunk_overlap >= chunk_size:
        chunk_overlap = chunk_size // 5

    step = chunk_size - chunk_overlap
    if step <= 0:
        step = chunk_size

    chunks: List[str] = []
    start = 0
    while start < len(runes):
        end   = min(start + chunk_size, len(runes))
        chunk = "".join(runes[start:end]).strip()
        if chunk:
            chunks.append(chunk)
        if end == len(runes):
            break
        start += step
    return chunks


if __name__ == "__main__":
    uvicorn.run("app:app", host="0.0.0.0", port=PORT, log_level="info")
