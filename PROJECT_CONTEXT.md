# OpsPilot — Project Context & Living Memory

> **Single Source of Truth** for architectural boundaries, service contracts, tech stack decisions, and current implementation progress. Update this file whenever meaningful system design or implementation state changes occur.

---

## 1. Projenin Amacı (Mission)
OpsPilot; doğal dilde verilen infrastructure / system administration görevlerini Linux sunucular üzerinde güvenli, denetlenebilir ve doğrulanabilir operasyonlara dönüştüren AI-assisted infrastructure operations platformudur.

OpsPilot basit bir “LLM → SSH → Shell command” botu **değildir**.
- **LLM**: Sadece reasoning, tool seçimi ve plan oluşturma yapar (untrusted probabilistic planner). Doğrudan shell erişimi kesinlikle yoktur.
- **Control Plane**: Sistemin tek otorite merkezidir (source of authority). Schemaları doğrular, policy engine çalıştırır, risk seviyesini hesaplar, approval kapılarını yönetir, state machine işletir ve audit trail yazar.
- **Server Agent**: Yönetilen sunucuda (Ubuntu 22.04 / 24.04) çalışan, Control Plane'e gRPC üzerinden outbound bağlanan, typed tool setine sahip kısıtlı ve güvenli çalıştırıcıdır (restricted executor).
- **Verification Engine**: Bir operasyonun başarısını exit code 0 veya LLM sözüne değil; deterministik socket, port, process ve HTTP probelarına göre doğrular (source of truth for success).

---

## 2. Aktif Mimari (Active Architecture)

```
+-------------------------------------------------------------+
|                      React Dashboard                        |
|  (Fleet, Live Timeline, Approval Gate, Runbooks, Audits)    |
+------------------------------+------------------------------+
                               | REST / SSE
                               v
+-------------------------------------------------------------+
|                   Go Control Plane (Port 8080/9090)         |
|  - Auth & RBAC (Admin, Operator, Viewer)                    |
|  - Task State Machine                                       |
|  - Policy Engine & Risk Evaluator (READ_ONLY/LOW/MED/HIGH)  |
|  - Approval Workflow Manager                                |
|  - Verification Engine                                      |
|  - Audit Trail & Sensitive Redaction                        |
|  - Fleet / Agent Registry (mTLS gRPC Hub)                   |
+---------------+-----------------------------+---------------+
                | HTTP / JSON                 | gRPC / mTLS
                v                             v
+-------------------------------+  +--------------------------+
|       Python AI Service       |  |     Go Server Agent      |
|  - FastAPI (Port 8000)        |  |  (Managed Linux Host)    |
|  - Ollama / Qwen Provider     |  |  - Capabilities Report   |
|  - Strict Pydantic Schemas    |  |  - Heartbeat & Metrics   |
|  - Prompt Injection Defense   |  |  - Typed Tool Executor   |
|  - Untrusted Output Isolation |  |  - Safe Fallback Parser  |
+-------------------------------+  +--------------------------+
                |
                v
       +-----------------+
       | PostgreSQL 16   |
       +-----------------+
```

---

## 3. Servislerin Sorumlulukları

| Servis | Dil / Teknoloji | Port | Sorumluluk |
| :--- | :--- | :--- | :--- |
| **Control Plane** | Go, PostgreSQL, pgx | 8080 (HTTP/SSE), 9090 (gRPC) | Yetkilendirme, State Machine, Policy Engine, Risk Değerlendirmesi, Approval, Verification, Audit |
| **Server Agent** | Go, gRPC (mTLS) | Outbound connection | Host keşfi, metrik toplama, typed tool çalıştırma, güvenli raw command fallback, process denetimi |
| **AI Service** | Python, FastAPI, Pydantic, Ollama | 8000 (HTTP) | Intent anlama, ortam keşif verilerini değerlendirme, yapısal plan (JSON) üretme, replanning |
| **Dashboard** | React, TypeScript, Vite, Tailwind | 3000 (HTTP) | Görsel yönetim, gerçek zamanlı timeline, plan onaylama, runbook çalıştırma, audit inceleme |
| **Database** | PostgreSQL 16 | 5432 | Görevler, geçiş logları, ajan kayıtları, onay kayıtları, runbooklar, audit kayıtları |

---

## 4. Kullanılan Teknolojiler ve Kütüphaneler

- **Control Plane**: Go 1.23+, `net/http` + Chi router, `pgx/v5` PostgreSQL driver, `golang-jwt/jwt/v5`, `google.golang.org/grpc`, `google.golang.org/protobuf`.
- **Server Agent**: Go 1.23+, gRPC client, crypto/tls, systemd D-Bus/CLI entegrasyonu, non-root runner.
- **AI Service**: Python 3.11+, FastAPI, Pydantic v2, HTTPX, Ollama client (default model: `qwen2.5:3b` / `qwen2.5:7b`).
- **Dashboard**: Vite, React 18, TypeScript, Tailwind CSS, Lucide Icons, EventSource (SSE).
- **Altyapı**: Docker Compose (merkezi servisler + Ubuntu 24.04 test node).

---

## 5. Kritik Tasarım Kararları (Architectural Guardrails)

1. **LLM Shell Erişimsizliği**: LLM doğrudan terminal komutu çalıştırmaz. Sadece tanımlı typed tool'ları parametreleriyle önerir.
2. **Untrusted Data Isolation**: Komut çıktıları, loglar ve dosya içerikleri LLM'e `"UNTRUSTED OBSERVATION DATA"` olarak beslenir. Prompt injection girişimleri sistem talimatı olarak kabul edilmez.
3. **Deterministik Doğrulama**: Görevin tamamlanma kriteri LLM'in cevabı veya exit code 0 değildir; ayrı çalışan doğrulama adımlarıdır (`systemctl is-active`, port check, HTTP probe).
4. **Defense-in-Depth Command Execution**: Raw command (`execute_command`) fallback gerektiğinde; AST tokenizer, tehlikeli ikili dosya denetimi (`rm -rf`, `dd`, `mkfs`, fork bombs), argument denetimi ve dizin kısıtlamaları uygulanır.
5. **Ajan Outbound gRPC**: Ajanlar sunucu üzerinde açık port dinlemek zorunda kalmaz; Control Plane'e doğru outbound mTLS gRPC tüneli açar.

---

## 6. Mevcut Geliştirme Durumu (Current Status)

- **Phase 0 (Proje Yapısı & Mimari & Dokümanlar)**: [COMPLETED]
- **Phase 1 (Go Control Plane Core, Auth, State Machine)**: [COMPLETED]
- **Phase 2 (Proto & Go Server Agent, Command Guard, Typed Tools)**: [COMPLETED]
- **Phase 3 (Python AI Service & Schemas, Prompt Defense)**: [COMPLETED]
- **Phase 4 (Policy Engine & Approval Flow, Risk Tiers)**: [COMPLETED]
- **Phase 5 (Verification Engine & Append-Only Audit Trail)**: [COMPLETED]
- **Phase 6 (Dashboard & Live SSE Timeline, Approvals, Runbooks)**: [COMPLETED]
- **Phase 7 (Runbook Sistemi - Reusable Deterministic Templates)**: [COMPLETED]
- **Phase 8 (Multi-Server Fleet - Parallel Orchestration)**: [COMPLETED]
- **Phase 9 (Benchmark Lab - Controlled Failure Scenarios & Runner)**: [COMPLETED]

---

## 7. Tamamlanan Başarılar & Doğrulamalar

1. **Control Plane Unit Tests**: Auth (JWT/Bcrypt), Policy Engine (READ_ONLY vs MEDIUM vs FORBIDDEN), and Task State Machine (Valid vs Illegal Transitions) %100 başarılı.
2. **Server Agent Unit Tests**: Command Guard (rm -rf /, mkfs, shutdown, fork bomb engelleme) ve File Tool (otomatik .bak ve rollback) %100 başarılı.
3. **AI Service Unit Tests**: Pydantic schema validation ve injection defense prompt testleri %100 başarılı.
4. **Dashboard Production Build**: Vite + React + Tailwind prod derlemesi sıfır hata ile tamamlandı (`dist/`).
5. **Multi-Platform Binary Builds**: `bin/control-plane.exe`, `bin/opspilot-agent.exe`, ve Linux ELF `bin/opspilot-agent` statik binary olarak derlendi.
6. **Benchmark Suite**: 5 adet kontrollü Linux arıza senaryosu (`benchmarks/scenarios`) ve test harness'ı (`benchmarks/runner.py`) oluşturuldu.
