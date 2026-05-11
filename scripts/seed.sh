#!/usr/bin/env bash
set -euo pipefail
curl -s -X POST http://localhost:8080/v1/events \
  -H 'Content-Type: application/json' \
  -d '{"tenant_id":"tenant_demo","user_id":"user_123","session_id":"seed","role":"user","content":"Eu prefiro Go para o core de produção e Python para evals."}'
