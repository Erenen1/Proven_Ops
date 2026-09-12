---
description: Always-on core engineering habits and safety protocols
always_on: true
---

# Core Engineering Principles

1. **Inspect Before Modifying**: Never edit a file or refactor logic without first reading the current implementation and understanding its dependencies.
2. **Verify Against Code, Not Assumptions**: When documentation and code conflict, verify the actual code and runtime behavior. Correct the stale documentation.
3. **Preserve Established Architecture & Patterns**: Follow repository conventions (e.g. Go Chi + pgx, React + Tailwind, Python FastAPI + Pydantic). Avoid introducing rogue frameworks or redundant abstractions.
4. **Least-Privilege & Defense-in-Depth**:
   - The LLM is an untrusted planner; never grant it direct shell execution authority.
   - Always prefer typed tools over raw shell execution.
   - Enforce non-root execution contexts where possible.
5. **No Secret Leakage**: Never commit credentials, private keys, tokens, or live passwords. Always use `.env.example`.
6. **Atomic Changes**: Keep edits focused and single-purpose. Do not mix unrelated refactors with bug fixes or feature work.
7. **Verify After Change**: Always run relevant tests, builds, and lint checks (`go test`, `pytest`, `npm run build`) before considering a task complete.
8. **Maintain Documentation Integrity**: When code changes affect architectural contracts, state machines, or workflows, update the corresponding documentation in `docs/context/`.
