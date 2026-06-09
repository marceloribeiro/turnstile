#!/usr/bin/env python3
"""Interactive OpenRouter chat through Turnstile — the answer streams back token
by token. OpenRouter is OpenAI-compatible, so this uses the OpenAI SDK; the only
change from a normal integration is base_url, which points at the Turnstile data
plane (/api/v1 → routed to the OpenRouter adapter)."""

import os
import sys

try:
    from dotenv import load_dotenv

    load_dotenv()
except ImportError:
    pass

from openai import OpenAI

BASE_URL = os.environ.get("TURNSTILE_BASE", "http://localhost:8080").rstrip("/") + "/api/v1"
API_KEY = os.environ.get("OPENROUTER_API_KEY")
MODEL = os.environ.get("MODEL", "openai/gpt-4o-mini")
SESSION = os.environ.get("TURNSTILE_SESSION", "openrouter-sample")

if not API_KEY:
    sys.exit("Set OPENROUTER_API_KEY (copy .env.example to .env and fill it in).")

client = OpenAI(base_url=BASE_URL, api_key=API_KEY)

print(f"Connected via Turnstile → {BASE_URL}  (model: {MODEL})")
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
        stream = client.chat.completions.create(
            model=MODEL,
            messages=[{"role": "user", "content": question}],
            stream=True,
            extra_headers={"X-Turnstile-Session": SESSION},
        )
        for chunk in stream:
            if chunk.choices and chunk.choices[0].delta.content:
                print(chunk.choices[0].delta.content, end="", flush=True)
        print("\n")
    except Exception as e:  # noqa: BLE001 — surface any provider/Turnstile error to the user
        print(f"\n[error] {e}\n")
