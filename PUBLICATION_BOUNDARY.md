# Publication Boundary

This repository is the public engineering surface.

Allowed:

- Source code, tests, interfaces, migrations, local-development examples, and operational docs.
- Placeholder environment variables and non-secret local demo values.
- Public-safe benchmark harnesses without private reports or proprietary datasets.

Excluded:

- `.living_memory_vault/`, local memory boxes, SQLite vaults, generated traces, and private project data.
- Research notes, private architecture analysis, private commercial material, and future revenue plans.
- Reference drops from other local projects unless explicitly reviewed for license and sensitivity.
- PDFs, screenshots, generated reports, caches, compiled binaries, and personal absolute paths.
- Any real API key, token, password, connection string, private SSH key, credential hash, or customer/user data.

Before publishing a new change, run a secret scan and review docs for strategic/private material.
