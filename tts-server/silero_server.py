import io
import logging
import re
from contextlib import asynccontextmanager

import numpy as np
import soundfile as sf
import torch
from fastapi import FastAPI, HTTPException
from fastapi.responses import StreamingResponse
from num2words import num2words
from pydantic import BaseModel

logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(message)s")
log = logging.getLogger("silero-tts")

LANGUAGE = "ru"
MODEL_ID = "v5_ru"
DEFAULT_SPEAKER = "kseniya"
SAMPLE_RATE = 48000

tts_model = None


def normalize_text(text: str) -> str:
    def replace_num(match):
        return num2words(int(match.group()), lang="ru")

    text = re.sub(r"\d+", replace_num, text)
    return text


@asynccontextmanager
async def lifespan(app: FastAPI):
    global tts_model
    log.info("Загружаю Silero TTS: language=%s, model=%s", LANGUAGE, MODEL_ID)

    device = torch.device("cpu")
    tts_model, _ = torch.hub.load(  # pyright: ignore[reportGeneralTypeIssues]
        repo_or_dir="snakers4/silero-models",
        model="silero_tts",
        language=LANGUAGE,
        speaker=MODEL_ID,
        trust_repo=True,  # pyright: ignore[reportArgumentType]
    )
    tts_model.to(device)

    log.info("Модель загружена. Голос по умолчанию: %s, sample_rate=%d", DEFAULT_SPEAKER, SAMPLE_RATE)
    yield
    log.info("Останавливаюсь...")


app = FastAPI(title="Silero TTS Local Server", lifespan=lifespan)


class TTSRequest(BaseModel):
    text: str
    speaker: str = DEFAULT_SPEAKER
    sample_rate: int = SAMPLE_RATE


@app.post("/tts")
async def synthesize(req: TTSRequest):
    if tts_model is None:
        raise HTTPException(status_code=503, detail="Модель ещё не загружена")

    text = req.text.strip()
    if not text:
        raise HTTPException(status_code=400, detail="Пустой текст")

    text = normalize_text(text)
    log.info("Синтез: %r (speaker=%s)", text, req.speaker)

    try:
        audio = tts_model.apply_tts(
            text=text,
            speaker=req.speaker,
            sample_rate=req.sample_rate,
        )
    except Exception as e:
        log.exception("Ошибка синтеза")
        raise HTTPException(status_code=500, detail=str(e))

    audio_np = audio.numpy() if isinstance(audio, torch.Tensor) else np.asarray(audio)

    buf = io.BytesIO()
    sf.write(buf, audio_np, req.sample_rate, format="WAV")
    buf.seek(0)

    return StreamingResponse(buf, media_type="audio/wav")


@app.get("/health")
async def health():
    return {
        "status": "ok",
        "model_loaded": tts_model is not None,
        "model": MODEL_ID,
        "sample_rate": SAMPLE_RATE,
        "default_speaker": DEFAULT_SPEAKER,
    }


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="127.0.0.1", port=9877, log_level="info")