#!/usr/bin/env python3
"""Interactive Gemini chat through Turnstile — streams the answer back. Gemini's
SDK doesn't take a base_url= kwarg like the others; you point it at a custom
endpoint via http_options. That endpoint is the Turnstile data plane, which routes
/v1beta/models/{model}:streamGenerateContent to the Gemini adapter."""

import os
import sys

try:
    from dotenv import load_dotenv

    load_dotenv()
except ImportError:
    pass

from google import genai
from google.genai import types

BASE_URL = os.environ.get("TURNSTILE_BASE", "http://localhost:8080").rstrip("/")
API_KEY = os.environ.get("GEMINI_API_KEY")
MODEL = os.environ.get("MODEL", "gemini-2.0-flash")
SESSION = os.environ.get("TURNSTILE_SESSION", "gemini-sample")

if not API_KEY:
    sys.exit("Set GEMINI_API_KEY (copy .env.example to .env and fill it in).")

client = genai.Client(
    api_key=API_KEY,
    http_options=types.HttpOptions(
        base_url=BASE_URL,
        # Groups this CLI's turns into one Turnstile session.
        headers={"X-Turnstile-Session": SESSION},
    ),
)

print(f"Connected via Turnstile → {BASE_URL}/v1beta/...:streamGenerateContent  (model: {MODEL})")
print("Type a question. Blank line or Ctrl-C to quit.\n")

while True:
    try:
        question = input("you> ").strip()
    except (EOFError, KeyboardInterrupt):
        print()
        break
    if not question:
        break

    print("assistant> ", end="", flush=True)
    try:
        for chunk in client.models.generate_content_stream(model=MODEL, contents=question):
            if chunk.text:
                print(chunk.text, end="", flush=True)
        print("\n")
    except Exception as e:  # noqa: BLE001 — surface any provider/Turnstile error to the user
        print(f"\n[error] {e}\n")
