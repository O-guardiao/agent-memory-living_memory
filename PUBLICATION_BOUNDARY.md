# Publication Boundary

This repository is the public engineering surface.

This tree is a sanitized staging area, not the private workspace. Copy files into
it intentionally; do not sync the full workspace into this folder.

Allowed:

- Source code, tests, interfaces, migrations, local-development examples, and operational docs.
- Public-safe configuration notes only. Do not publish `.env*` files, including `.env.example`.
- Public-safe tests and benchmark harnesses without private reports, proprietary datasets, private claims, or startup strategy.

Excluded:

- `.living_memory_vault/`, local memory boxes, SQLite vaults, generated traces, and private project data.
- Research notes, private architecture analysis, private commercial material, and future revenue plans.
- Reference drops from other local projects unless explicitly reviewed for license and sensitivity.
- PDFs, screenshots, generated reports, caches, compiled binaries, and personal absolute paths.
- Any `.env*` file, real API key, token, password, connection string, private SSH key, credential hash, or customer/user data.

Before publishing a new change, run `scripts/check_public_release.ps1` from the private workspace and review docs for strategic/private material.
