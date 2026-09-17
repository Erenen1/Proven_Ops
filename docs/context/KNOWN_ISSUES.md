# Known Issues & Technical Debt

This document tracks project-level technical debt, known operational limitations, and security considerations. It is not a bug tracker copy; resolved items are removed or marked as addressed.

---

## 1. Technical Debt & Operational Considerations

### Issue 1: Automated mTLS Certificate Lifecycle & Renewal
- **Severity**: Low (Production Hardening)
- **Current State**: Agent registration is authenticated via a cryptographically secure bootstrap token, establishing a secure connection. Full production mutual TLS (mTLS) with automated X.509 cert issuance and periodic CRL checks is planned for v1.0.
- **Workaround**: For development and staging environments, token-authenticated gRPC with TLS transport encryption is active.

### Issue 2: Cross-Compilation for Linux ARM64
- **Severity**: Low (Platform Expansion)
- **Current State**: `scripts/build-binaries.ps1` builds Linux amd64 and Windows binaries. AWS Graviton and ARM-based edge servers require `GOARCH=arm64`.
- **Remediation**: Add `GOARCH=arm64` build target to `Makefile` and `build-binaries.ps1`.

### Issue 3: Offline Ollama Fallback Coverage
- **Severity**: Informational
- **Current State**: The AI Service includes heuristic fallback planning for standard benchmark operations (Nginx on custom ports, Docker host bootstrap, system telemetry) if the local Ollama instance is downloading weights or offline.
- **Remediation**: For non-standard custom prompts without Ollama running, the AI Service will return an error until the model is booted.

### Issue 4: Docker-in-Docker CI Integration
- **Severity**: Low (CI Enhancement)
- **Current State**: CI runs unit tests for Go, Python, and TypeScript. Running the live multi-node agent lab (`agent/Dockerfile`) in GitHub Actions requires privileged Docker runners (`--privileged`).
- **Workaround**: Real multi-node verification is performed via `make validate-production` locally or on dedicated staging VMs.
