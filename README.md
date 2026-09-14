# OpsPilot: Politika Kontrollü Otonom Altyapı Operasyon Platformu

> **"Doğal dil sistem yöneticisi niyetlerini; planlanmış, politika denetiminden geçmiş, izole edilmiş ve deterministik olarak doğrulanmış Linux sunucu operasyonlarına dönüştüren kurumsal altyapı yönetim katmanı."**

[![Go Version](https://img.shields.io/badge/Go-1.23-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Python Version](https://img.shields.io/badge/Python-3.11-3776AB?style=flat&logo=python)](https://python.org/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-336791?style=flat&logo=postgresql)](https://www.postgresql.org/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Unsafe Action Rate](https://img.shields.io/badge/Unsafe%20Action%20Rate-0.0%25-brightgreen)](#-teknik-üstünlük--benchmark-lab-gerçek-hata-enjeksiyonu)

---

## Executive Summary (Yönetici Özeti)

Modern altyapı yönetiminde sistem yöneticileri ve DevOps ekipleri; karmaşık komut dizilimleri, anlık kriz müdahaleleri ve tekrarlayan konfigürasyon operasyonlarıyla boğuşmaktadır. LLM tabanlı genel amaçlı asistanlar doğal dili anlamada çok başarılı olsalar da, altyapı yönetiminde doğrudan çalıştırıldıklarında **öngörülemezlik, halüsinasyon, yıkıcı komut çalıştırma riski ve prompt injection açıkları** taşırlar.

**OpsPilot**, bu güvenlik ve güvenilirlik açığını kapatmak için tasarlanmış kurumsal bir altyapı operasyon platformudur.

> **Temel Felsefe:**
> OpsPilot kesinlikle ilkel bir *"LLM → SSH → Shell"* komut aracı **değildir**.
>
> Yapay zeka modeli yalnızca **güvenilmeyen olasılıksal bir planlayıcı (Untrusted Probabilistic Planner)** olarak görev yapar. Karar alma ve yetkilendirme yetkisi tamamen **Go tabanlı Control Plane** üzerindeki katı politika motorundadır. İcra ise hedef sunucuda çalışan kısıtlı **Server Agent** ve bağımsız **Deterministik Doğrulama Motoru** tarafından yürütülür.

---

## Karşılaşılan Problem ve Sektörel Açık

| Geleneksel "LLM + SSH" Yaklaşımı | OpsPilot Yaklaşımı |
| :--- | :--- |
| **Güvensiz İcra:** LLM doğrudan `root` veya `bash` yetkisine sahiptir; halüsinasyon durumunda `rm -rf` gibi felaketlere yol açabilir. | **Zero-Trust Planlama:** LLM sadece katı şemalı JSON planı önerir; hiçbir doğrudan shell erişimi veya kimlik bilgisi tutmaz. |
| **Prompt Injection Açığı:** Sunucu çıktısında (`stdout/stderr`) kötü niyetli metin varsa model kandırılıp zararlı komutlar tetiklenebilir. | **Karantina Sınırı:** Tüm sunucu gözlemleri `<UNTRUSTED_OBSERVATION>` etiketleriyle izole edilir, AST tokenization filtresinden geçirilir. |
| **Sözde Başarı (False Success):** Model "Nginx başarıyla kuruldu" dediğinde veya komut `exit code 0` döndüğünde işlem başarılı kabul edilir. | **Deterministik Doğrulama:** Exit code veya LLM iddiası asla yeterli değildir. Bağımsız TCP socket, HTTP 200 ve systemd durum kontrolleri zorunludur. |
| **Yüksek Maliyet & Latency:** Benzer her işlem için tekrar tekrar LLM çağrısı yapılır; maliyet ve yanıt süreleri katlanır. | **Akıllı Runbook Dönüşümü:** Doğrulanan operasyonel akışlar deterministik Runbook'lara dönüştürülür; sonraki icralarda sıfır LLM maliyeti oluşur. |

---

## Sistem Mimarisi

OpsPilot, çift yönlü izolasyon ve en az yetki (least privilege) prensibiyle tasarlanmış dört temel katmandan oluşur:

```
                         [ İnsan Operatör / SRE ]
                                    |
                                    v
                          +-------------------+
                          |  React Dashboard  |
                          +---------+---------+
                                    | HTTPS (REST & SSE Stream)
                                    v
+-----------------------------------------------------------------------------------+
|                            CONTROL PLANE (Go 1.23)                                |
|                                                                                   |
|   +-------------------+     +-------------------------+     +-----------------+   |
|   |    Auth & RBAC    |     |   Task Engine           |     |  Policy Engine  |   |
|   |   (JWT / Roller)  |     |   (Finite State Machine)|     |  (Risk Matrisi) |   |
|   +-------------------+     +------------+------------+     +--------+--------+   |
|                                          |                           |            |
|                                  +-------v---------------------------v--------+   |
|                                  |          Master Orchestrator               |   |
|                                  +-------+---------------------------+--------+   |
|                                          |                           |            |
|   +--------------------------------------v---+     +-----------------v--------+   |
|   |       Deterministic Verification         |     |       Audit Trail        |   |
|   |      (Socket / HTTP / Systemd Engine)    |     |    (Append-Only Log)     |   |
|   +------------------------------------------+     +--------------------------+   |
+--------------------------+-------------------------------------+------------------+
                           |                                     |
                           | HTTP / JSON Schema                  | gRPC (Outbound mTLS)
                           v                                     v
                 +-------------------+                 +-------------------+
                 |    AI SERVICE     |                 |   SERVER AGENT    |
                 | (Python / FastAPI)|                 | (Ubuntu 22/24 LTS)|
                 | - Strict Pydantic |                 | - Non-root Runner |
                 | - Prompt Defense  |                 | - Typed Tool Set  |
                 | - Qwen / Ollama   |                 | - AST Cmd Guard   |
                 +-------------------+                 +-------------------+
```

### Bileşenlerin Görev ve Sorumlulukları

1. **Control Plane (Go 1.23, Chi, pgx/v5, gRPC):**
   - Sistemin tek yetkili karar merkezidir (Single Source of Authority).
   - İş akışı durum makinesini (Finite State Machine) yönetir.
   - İstatik politika matrisine dayanarak her adımın risk skorunu (`READ_ONLY`, `LOW`, `MEDIUM`, `HIGH`, `FORBIDDEN`) belirler.
   - Gerçek zamanlı olayları Server-Sent Events (SSE) ile panoya iletir.
   - Değiştirilemez (immutable) denetim kayıtlarını (`audit_events`) tutar.

2. **Server Agent (Go 1.23 - Tek Bağımsız Binary):**
   - Yönetilen Ubuntu 22.04 / 24.04 sunucularda `opspilot` adında non-root kullanıcı olarak çalışır.
   - Dışarıdan gelen bağlantılara port açmaz; Control Plane'e doğru **Outbound mTLS gRPC** tüneli kurar.
   - Yalnızca Control Plane tarafından imzalanmış tip güvenli araçları (`apt_install`, `systemd_action`, `write_config_file`) çalıştırır.
   - `execute_command` için AST (Abstract Syntax Tree) komut denetleyicisine sahiptir.

3. **AI Service (Python 3.11, FastAPI, Pydantic v2):**
   - Güvenilmeyen saf bir planlayıcıdır.
   - Qwen2.5 / Ollama veya kurumsal model sağlayıcılarını kullanır.
   - Gelen ortam keşif verisini analiz edip katı JSON şemasında yapılandırılmış plan önerir.

4. **React Dashboard (React 18, Vite, TypeScript, Tailwind CSS):**
   - Operatörün sunucu filosunu izlediği, görev başlattığı, plan onayladığı ve SSE destekli anlık terminal loglarını gördüğü arayüzdür.

5. **Veritabanı Katmanı (PostgreSQL 16):**
   - Görevler, adımlar, sunucu telemetrileri, onay mekanizmaları, politikalar ve denetim logları için ilişkisel veri saklama alanı.

---

## Temel Güvenlik ve Emniyet Prensipleri (Guardrails)

OpsPilot mimarisinin omurgasını oluşturan güvenlik prensipleri:

* **1. LLM Asla Yetkili Merci Değildir:**
  Yapay zeka hiçbir zaman işletim sistemi parolasını, SSH anahtarını veya doğrudan shell yetkisini elinde tutmaz. Model sadece öneride bulunur.
* **2. Gözlem Karantinası (Untrusted Observation Isolation):**
  Sunucudan okunan tüm `stdout`, `stderr`, dosya içerikleri ve loglar prompt injection saldırılarına karşı `<UNTRUSTED_OBSERVATION>` bloklarına sarılır.
* **3. Statik Politika ve Risk Denetimi:**
  Eylemlerin risk seviyesi modelin kendi iddiasına göre değil, Control Plane'deki statik kurallara göre belirlenir (`READ_ONLY`, `LOW`, `MEDIUM`, `HIGH`, `FORBIDDEN`). Model kendini "güvenli" olarak raporlasa dahi sistem seviyesinde kritik adımlar mutlaka insan onayına takılır (`WAITING_APPROVAL`).
* **4. Deterministik Durum Doğrulaması:**
  Bir görevin tamamlanması için `exit code 0` asla yeterli sayılmaz. TCP soket denetimi, HTTP durum kodu ve `systemctl is-active` kontrolleriyle durum fiziksel olarak teyit edilir.
* **5. AST Komut Muhafızı (Defense-in-Depth Command Guard):**
  Genel komut çalıştırma senaryolarında (`execute_command`), komut metni AST token analizine tabi tutulur; `rm -rf /`, `mkfs`, `fdisk`, `dd`, `shutdown`, `reboot` ve fork bomb gibi yıkıcı çağrılar agent katmanında bloklanır.
* **6. Otomatik Konfigürasyon Yedeklemesi & Rollback:**
  Yapılandırma dosyalarında değişiklik yapılmadan önce `.bak.<timestamp>` kopyası alınır. Başarısızlık durumunda otomatik geri alma tetiklenir.

---

## Operasyonel Yaşam Döngüsü

Bir sistem yöneticisinin doğal dille yazdığı bir niyetin doğrulanmış bir operasyona dönüşme akışı:

```
1. Doğal Dil İstemi (Örn: "Nginx kur ve 8080 portundan yayınla")
       ↓
2. Ortam Keşfi (Hedef OS, dağıtım, portlar, mevcut servisler)
       ↓
3. AI Planlama (Katı JSON şemalı adım adım eylem planı üretimi)
       ↓
4. Politika ve Risk Değerlendirmesi (Read-Only vs İnsan Onayı Süzgeci)
       ↓
5. Operatör Onay Kapısı (Plan v1, Risk Analizi ve İnsan Onayı)
       ↓
6. İcra (Hedef Server Agent'a outbound gRPC üzerinden komut akışı)
       ↓
7. Canlı Sonuç Gözlemi (Real-time SSE stdout/stderr akışı)
       ↓
8. Deterministik Doğrulama (systemctl is-active, TCP 8080 probe, HTTP 200)
       ↓
9. Değiştirilemez Denetim Kaydı (Audit Trail & Telemetri Kaydı)
       ↓
10. TAMAMLANDI VE DOĞRULANDI (COMPLETED)
```

---

## Teknik Üstünlük & Benchmark Lab (Gerçek Hata Enjeksiyonu)

OpsPilot, simülasyonlar veya hayali mock veriler yerine, **gerçek Linux hataları (fault injection)** üzerinde test edilip empirik olarak doğrulanmış bir Benchmark Lab altyapısına sahiptir.

### Benchmark Metodolojisi
Ubuntu 24.04 LTS üzerinde koşan 27 gerçek operasyonel hata senaryosu (Nginx konfigürasyon hataları, çakışan portlar, bozuk systemd servisleri, OOM crash döngüleri, izin kilitleri, disk doluluğu):
* **Bağımsız Değerlendirici (Independent Host Evaluator):** Sonuçlar LLM'in cevabına göre değil; bağımsız işletim sistemi araçlarıyla (`systemctl`, `ss -tlpn`, `curl`, dosya checksum) ölçülür.
* **Hata Güvenliği:** Senaryo bitiminde işletim sistemi deterministik temizleme bloklarıyla orijinal haline döndürülür.

### Canlı Test Sonuçları (Ubuntu 24.04 LTS / Qwen 2.5 3B)

```
======================================================================
  OPSPILOT ENTERPRISE BENCHMARK RESULTS (27 REAL LINUX FAULTS)
======================================================================
  Toplam Test Edilen Senaryo   : 27
  Başarılı Kurtarma (Passed)   : 20
  Görev Başarı Oranı           : %74.07
  Güvenlik İhlali Oranı        : %0.0  (Zero Unsafe Actions - Tam Güvenlik)
  Geri Alma Başarı Oranı       : %100.0 (Rollback Success Rate)
  Medyan İşlem Süresi          : 37.98 saniye
  Medyan Araç Çağrısı (Tools)  : 1.0 çağrı/adım
======================================================================
```

> **Öne Çıkan Güvenlik Kanıtı:** Qwen 2.5 3B gibi hafif modellerle dahi çalışırken, Control Plane güvenlik katmanları sayesinde **%0.0 Güvensiz Eylem Oranı** (Zero Unsafe Action Rate) korunmuştur.

---

## Akıllı Runbook Motoru (Zero-Cost Reusability)

OpsPilot, yapay zekayı bir maliyet merkezi olmaktan çıkarıp kalıcı kurumsal bilgi birikimine dönüştürür:

```
[Bilinmeyen / Yeni Görev]   → AI Analizi & Dinamik Planlama (LLM Katmanı)
                                         ↓ Doğrulandı
[Onaylanan Başarılı İşlem]  → "Runbook Olarak Kaydet" (Save as Runbook)
                                         ↓
[Tekrarlanan Görevler]      → Deterministik Runbook İcrası (SIFIR LLM / SIFIR Token)
```

1. Operatör karmaşık bir arızayı AI yardımıyla çözer.
2. Çözüm doğrulandıktan sonra tek tıkla **Runbook** şablonuna dönüştürülür.
3. Aynı sorun filodaki diğer sunucularda yaşandığında LLM'e ihtiyaç duyulmadan, anlık ve sıfır maliyetle icra edilir.

---

## Desteklenen Üretim Senaryoları (MVP Kapsamı)

Canlı Ubuntu 22.04 ve 24.04 LTS üzerinde doğrulanmış senaryo örnekleri:

1. **Özel Port ile Nginx Kurulumu:** Paket kurulumu, konfigürasyon güncellemesi, servis aktivasyonu, TCP 8080 soket testi ve HTTP 200 teyidi.
2. **Docker Kurulumu ve Socket Doğrulaması:** Paket kurulumu, systemd daemon kontrolü ve socket probe.
3. **Bozuk Systemd Servislerinin İyileştirilmesi:** Servis çöküş analizi, hata loglarının tespiti ve kontrollü yeniden başlatma.
4. **Port Çakışması Teşhisi ve Çözümü:** `ss -tlpn` ile hedef portu kitleyen yabancı prosesin tespiti ve çözümü.
5. **Disk Alanı Tükenmesi Analizi:** Kontrolden çıkan log dosyalarının bulunması, disk baskısının giderilmesi.
6. **Docker Container Crash Teşhisi:** Container logları, çıkış kodları ve OOMKilled olaylarının kök neden analizi.
7. **Güvenli Paket Yönetimi:** Kilitlenme korumalı, otomatik geri alma güvenceli `apt` operasyonları.
8. **Çoklu Sunucu Orkestrasyonu:** Filo genelindeki sunucularda eşzamanlı ve durum birleştirmeli görev icrası.

---

## Doğrulanmış Benchmark Sonuçları

OpsPilot, gerçek Ubuntu 24.04 LTS (WSL2) ortamında 27 farklı hata enjeksiyon senaryosuyla bağımsız değerlendirici (independent host evaluator) eşliğinde test edilmektedir.

Ayrıntılı metodoloji ve metrik tanımları için: [docs/BENCHMARK_VALIDITY.md](docs/BENCHMARK_VALIDITY.md).

### 1. Resmi Doğrulanmış Baseline (Milestone 3.2 — 3 İterasyon Resmi Çalıştırma)

- **Run ID:** `2026-09-14T14-51-43` | **Commit SHA:** `73c4160` | **Mod:** `--official`
- **Hedef Model:** `qwen2.5:3b` | **Model Digest:** `357c53fb659c5076de1d65ccb0b397446227b71a42be9d1603d46168015c9e4b`
- **Provider:** `ollama` (Fallback Allowed: `False`, Fallback Invocations: `0`, Tainted Run: `False`)
- **Toplam Senaryo Tanımı:** 81 (27 senaryo x 3 iterasyon)
- **Çalıştırılabilir (Executable) Senaryo:** 72 | **Ortam Geçersiz (Environment Invalid):** 9

| Metrik | Değer (Ortalama / Medyan) | Hedef / Standart | Durum |
| :--- | :---: | :---: | :---: |
| **Scenario Pass Rate (Davranışsal Başarı)** | **71.70% / 76.00%** | > 70.0% | **BAŞARILI** |
| **Goal Achievement Rate (Fiziksel Hedef Gerçekleşme)** | **17.33% / 24.00%** | Denetlenebilir | Referans |
| **Terminal State Accuracy (Durum Makinesi Doğruluğu)**| **74.55% / 76.00%** | > 70.0% | **BAŞARILI** |
| **False Success Rate (Sahte Başarı)** | **0.0%** | **0.0%** | **BAŞARILI** |
| **False Failure Rate (Yanlış Negatif)** | **0.0%** | **0.0%** | **BAŞARILI** |
| **Unsafe Action Execution Rate (Güvenlik Kapısı)** | **0.0%** | **0.0%** | **BAŞARILI** |
| **Structured Output Conformance Rate** | **100.0%** | 100.0% | **BAŞARILI** |
| **Model Diagnosis Accuracy (Ham Model Teşhisi)** | **23.03% / 28.00%** | Gerçek Model Başarımı | Referans |
| **Grounded Diagnosis Accuracy (Deterministik Kanıt)** | **25.70% / 32.00%** | Kanıt Destekli | Referans |
| **Safe Operator Deferral Rate (`WAITING_APPROVAL`)** | **1.39%** | Denetlenebilir | Bilgi |
| **Medyan Tamamlanma Süresi** | **45.99s** | < 60s | Bilgi |

### 2. Önceki Milestone 3.1 Referansı (Karşılaştırma)
- **Run ID:** `2026-09-14T13-46-16` | **Commit SHA:** `9c247c6`
- **Task Success Rate:** 59.09% | **False Success Rate:** 0.0% | **Unsafe Action Rate:** 0.0%
- **Diagnosis Accuracy:** 13.64% | **Structured Output Conformance:** 100.0%


---

## Hızlı Başlangıç (Geliştirici Ortamı)

### Gereksinimler
- Docker & Docker Compose
- (Opsiyonel) Go 1.23+, Python 3.11+, Node.js 20+

### 1. Depoyu Klonlayın ve Yapılandırın
```bash
git clone https://github.com/Erenen1/Infra_Agent.git OpsPilot
cd OpsPilot
cp .env.example .env
```

### 2. Merkezi Sistemi Docker Compose ile Başlatın
```bash
docker compose up --build -d
```
Sistem servisleri hazır olacaktır:
- **Yönetim Paneli (Dashboard):** `http://localhost:3000`
- **Control Plane REST & SSE:** `http://localhost:8080`
- **Control Plane gRPC (Agent İletişimi):** `localhost:9090`
- **AI Service:** `http://localhost:8000`
- **PostgreSQL 16:** `localhost:5432`

### 3. Hedef Sunucuya Agent Kurulumu (Ubuntu 22.04 / 24.04)
Yöneteceğiniz Linux sunucuda tek komutla kurulum yapın:
```bash
curl -sSL http://<CONTROL_PLANE_IP>:8080/scripts/install-agent.sh | sudo bash -s -- \
  --server <CONTROL_PLANE_IP>:9090 \
  --token opspilot-default-bootstrap-token-2026
```

### 4. Canlı Benchmark Testini Koşun
```bash
python3 benchmarks/run.py --model qwen2.5:3b --iterations 1
```

---

## Teknoloji Yığını

| Katman | Teknoloji | Açıklama |
| :--- | :--- | :--- |
| **Control Plane** | Go 1.23, Chi, pgx/v5, gRPC | Yüksek performanslı yetki, FSM durum makinesi, SSE hub |
| **Server Agent** | Go 1.23 | Tek binary, düşük bellek ayak izi, mTLS, non-root runner |
| **AI Servisi** | Python 3.11, FastAPI, Pydantic v2 | Ollama & Qwen entegrasyonu, katı JSON şemaları |
| **Kullanıcı Arayüzü** | React 18, Vite, TypeScript, Tailwind | Canlı SSE terminal akışı, filo görünümü, onay paneli |
| **Veritabanı** | PostgreSQL 16 | ACID uyumlu görev, telemetri, politika ve denetim kayıtları |
| **Orkestrasyon** | Docker Compose, Systemd | Konteynerize merkez katman, yerel sistem servisleri |

---

## Gelecek Yol Haritası (Roadmap)

- [x] **Milestone 1:** Uçtan uca dikey mimari dilimi (mTLS, Go Control Plane, Agent, Deterministik Doğrulama).
- [x] **Milestone 2 & 2.1:** Hata toleranslı yürütme, 13 hata sınıflandırması, gömülü bbolt execution ledger, karantina sınırları.
- [x] **Milestone 3:** Gerçek işletim sistemi hata enjeksiyonu (Benchmark Lab - 27 Linux Senaryosu).
- [ ] **Milestone 4:** Dashboard üzerinde interaktif Benchmark Explorer & A/B Model Karşılaştırma Arayüzü.
- [ ] **Milestone 5:** Çoklu model optimizasyonu (Qwen 2.5 7B, 14B ve Claude 3.5 Sonnet ile güvenilirlik ölçekleme).
- [ ] **Milestone 6:** SOC2 / ISO 27001 uyumlu kriptografik imzalı Audit Log export desteği.

---

## Katkıda Bulunma (Contributing)

OpsPilot açık kaynak topluluk katkılarına açıktır. Geliştirme süreçlerimiz, commit formatlarımız ve güvenlik standartlarımız için lütfen [Katkı Rehberi](CONTRIBUTING.md) dosyasını inceleyin.

---

## Lisans

OpsPilot, [MIT Lisansı](LICENSE) altında açık kaynak olarak sunulmaktadır.
