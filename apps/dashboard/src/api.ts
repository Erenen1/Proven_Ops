import { Agent, Task, TaskTransition, VerificationResult, Runbook, RunbookExecutionResult, AuditEvent } from './types';

const API_BASE = '/api/v1';

export async function fetchAgents(): Promise<Agent[]> {
  const res = await fetch(`${API_BASE}/agents`);
  if (!res.ok) throw new Error('Failed to fetch agents');
  return res.json();
}

export async function fetchTasks(): Promise<Task[]> {
  const res = await fetch(`${API_BASE}/tasks`);
  if (!res.ok) throw new Error('Failed to fetch tasks');
  return res.json();
}

export async function fetchTaskDetail(id: string): Promise<{
  task: Task;
  history: TaskTransition[];
  verifications: VerificationResult[];
}> {
  const res = await fetch(`${API_BASE}/tasks/${id}`);
  if (!res.ok) throw new Error('Failed to fetch task details');
  return res.json();
}

export async function createTask(payload: {
  title?: string;
  prompt: string;
  target_agent_ids: string[];
}): Promise<Task> {
  const res = await fetch(`${API_BASE}/tasks`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || 'Failed to create task');
  }
  return res.json();
}

export async function approveTask(id: string, planVersion: number, notes = ''): Promise<void> {
  const res = await fetch(`${API_BASE}/tasks/${id}/approve`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ plan_version: planVersion, notes }),
  });
  if (!res.ok) throw new Error('Failed to approve task');
}

export async function rejectTask(id: string, planVersion: number, notes = ''): Promise<void> {
  const res = await fetch(`${API_BASE}/tasks/${id}/reject`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ plan_version: planVersion, notes }),
  });
  if (!res.ok) throw new Error('Failed to reject task');
}

export async function fetchRunbooks(): Promise<Runbook[]> {
  const res = await fetch(`${API_BASE}/runbooks`);
  if (!res.ok) throw new Error('Failed to fetch runbooks');
  return res.json();
}

export async function createRunbook(payload: {
  task_id: string;
  slug: string;
  title: string;
  description: string;
}): Promise<Runbook> {
  const res = await fetch(`${API_BASE}/runbooks`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  if (!res.ok) throw new Error('Failed to create runbook');
  return res.json();
}

export async function dryRunRunbook(
  id: string,
  parameters: Record<string, string>
): Promise<RunbookExecutionResult> {
  const res = await fetch(`${API_BASE}/runbooks/${encodeURIComponent(id)}/dry-run`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ parameters }),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || 'Failed to simulate runbook');
  }
  return res.json();
}

export async function executeRunbook(
  id: string,
  payload: {
    target_agent_ids?: string[];
    parameters: Record<string, string>;
    dry_run?: boolean;
  }
): Promise<Task> {
  const res = await fetch(`${API_BASE}/runbooks/${encodeURIComponent(id)}/execute`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || 'Failed to execute runbook');
  }
  return res.json();
}

export async function fetchAudit(limit = 50): Promise<AuditEvent[]> {
  const res = await fetch(`${API_BASE}/audit?limit=${limit}`);
  if (!res.ok) throw new Error('Failed to fetch audit logs');
  return res.json();
}

