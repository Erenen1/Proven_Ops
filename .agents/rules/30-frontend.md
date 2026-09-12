---
description: Frontend engineering standards for React Dashboard
globs: ["apps/dashboard/**"]
---

# Frontend Engineering Conventions

## 1. Stack & Tools
- **Framework**: React 18 with TypeScript and Vite bundler.
- **Styling**: Tailwind CSS (dark mode default with `slate-950` backgrounds).
- **Icons**: `lucide-react`.

## 2. Design Principles
- **Clean & Technical**: The UI is designed for senior SREs and platform engineers. Prioritize density, clear typography (`JetBrains Mono` for code/ports), and high visual contrast.
- **No Superfluous Animations**: Avoid bouncing or distracting motion. Only use subtle status pulses (e.g. amber for pending approvals, emerald for online nodes).

## 3. Real-Time Streaming (SSE)
- Use standard browser `EventSource` to stream live updates from `/api/v1/tasks/:id/events` and `/api/v1/events`.
- Always close the `EventSource` in the `useEffect` cleanup return callback to prevent connection leaks.

## 4. State & Types
- All API entities must align with definitions in `src/types.ts`.
- Avoid arbitrary `any` casts; keep request payloads and component props strictly typed.
- Component structure must maintain clean modular separation between views (`Overview`, `Fleet`, `Tasks`, `Approvals`, `Runbooks`, `Audit`, `Policies`) and modals (`TaskDetailModal`, `NewTaskModal`).
