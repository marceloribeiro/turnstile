#!/usr/bin/env python3
"""Interactive Anthropic chat through Turnstile — the answer streams back token by
token. The only change from a normal Anthropic integration is base_url, which
points at the Turnstile data plane; the SDK still sends x-api-key and
anthropic-version, which Turnstile uses to route to the Anthropic adapter
(/v1/messages)."""

import os
import sys

try:
    from dotenv import load_dotenv

    load_dotenv()
except ImportError:
    pass

from anthropic import Anthropic

BASE_URL = os.environ.get("TURNSTILE_BASE", "http://localhost:8080").rstrip("/")
API_KEY = os.environ.get("ANTHROPIC_API_KEY")
MODEL = os.environ.get("MODEL", "claude-3-5-haiku-latest")
SESSION = os.environ.get("TURNSTILE_SESSION", "anthropic-sample")

if not API_KEY:
    sys.exit("Set ANTHROPIC_API_KEY (copy .env.example to .env and fill it in).")

client = Anthropic(base_url=BASE_URL, api_key=API_KEY)

print(f"Connected via Turnstile → {BASE_URL}/v1/messages  (model: {MODEL})")
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
        with client.messages.stream(
            model=MODEL,
            max_tokens=1024,
            messages=[{"role": "user", "content": question}],
            extra_headers={"X-Turnstile-Session": SESSION},
        ) as stream:
            for text in stream.text_stream:
                print(text, end="", flush=True)
        print("\n")
    except Exception as e:  # noqa: BLE001 — surface any provider/Turnstile error to the user
        print(f"\n[error] {e}\n")
