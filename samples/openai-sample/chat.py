#!/usr/bin/env python3
"""Interactive OpenAI chat through Turnstile — the answer streams back token by
token. The only thing that differs from a normal OpenAI integration is base_url:
it points at the Turnstile data plane instead of api.openai.com."""

import os
import sys

try:
    from dotenv import load_dotenv

    load_dotenv()
except ImportError:
    pass

from openai import OpenAI

BASE_URL = os.environ.get("TURNSTILE_BASE", "http://localhost:8080").rstrip("/") + "/v1"
API_KEY = os.environ.get("OPENAI_API_KEY")
MODEL = os.environ.get("MODEL", "gpt-4o-mini")
SESSION = os.environ.get("TURNSTILE_SESSION", "openai-sample")

if not API_KEY:
    sys.exit("Set OPENAI_API_KEY (copy .env.example to .env and fill it in).")

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
            # Groups this CLI's turns into one Turnstile session.
            extra_headers={"X-Turnstile-Session": SESSION},
        )
        for chunk in stream:
            if chunk.choices and chunk.choices[0].delta.content:
                print(chunk.choices[0].delta.content, end="", flush=True)
        print("\n")
    except Exception as e:  # noqa: BLE001 — surface any provider/Turnstile error to the user
        print(f"\n[error] {e}\n")
