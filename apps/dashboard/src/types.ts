export type TaskStatus =
  | 'CREATED'
  | 'DISCOVERING'
  | 'PLANNING'
  | 'WAITING_APPROVAL'
  | 'EXECUTING'
  | 'OBSERVING'
  | 'VERIFYING'
  | 'REPLANNING'
  | 'COMPLETED'
  | 'PARTIAL_SUCCESS'
  | 'FAILED'
  | 'ROLLED_BACK'
  | 'CANCELLED'
  | 'TIMEOUT';

export type RiskLevel = 'READ_ONLY' | 'LOW' | 'MEDIUM' | 'HIGH' | 'FORBIDDEN';

export interface AgentMetrics {
  cpu_usage_percent: number;
  memory_usage_bytes: number;
  memory_total_bytes: number;
  disk_usage_percent: number;
  load_avg_1m: number;
  active_tasks: number;
  recorded_at: string;
}

export interface Agent {
  id: string;
  hostname: string;
  ip_address: string;
  os: string;
  distribution: string;
  version: string;
  architecture: string;
  status: 'online' | 'offline' | 'busy';
  environment: string;
  capabilities: string[];
  last_heartbeat?: string;
  last_metrics?: AgentMetrics;
  created_at: string;
}

export interface VerificationStrategy {
  check_type: string;
  target: string;
  expected?: string;
  timeout_sec: number;
}

export interface TaskStep {
  id: string;
  task_id: string;
  step_order: number;
  action: string;
  arguments: Record<string, any>;
  risk_level: RiskLevel;
  requires_approval: boolean;
  verification_strategy?: VerificationStrategy;
  status: 'PENDING' | 'RUNNING' | 'SUCCESS' | 'FAILED' | 'SKIPPED';
  exit_code?: number;
  stdout?: string;
  stderr?: string;
  started_at?: string;
  finished_at?: string;
}

export interface TaskTransition {
  id: number;
  task_id: string;
  from_status: TaskStatus;
  to_status: TaskStatus;
  reason: string;
  triggered_by?: string;
  transitioned_at: string;
}

export interface VerificationResult {
  id: string;
  task_id: string;
  step_id?: string;
  agent_id: string;
  check_type: string;
  target: string;
  passed: boolean;
  details: Record<string, any>;
  verified_at: string;
}

export interface AIPlanData {
  goal: string;
  reasoning: string;
  steps: Array<{
    id: string;
    action: string;
    arguments: Record<string, any>;
    reason: string;
    suggested_risk: RiskLevel;
    verification_strategy?: VerificationStrategy;
  }>;
  overall_verification?: VerificationStrategy[];
}

export interface Task {
  id: string;
  title: string;
  prompt: string;
  status: TaskStatus;
  created_by?: string;
  target_agent_ids: string[];
  plan_version: number;
  planVersion?: number;
  ai_plan?: AIPlanData;
  risk_level: RiskLevel;
  error_message?: string;
  execution_summary?: string;
  steps?: TaskStep[];
  created_at: string;
  updated_at: string;
  completed_at?: string;
}

export interface Approval {
  id: string;
  task_id: string;
  plan_version: number;
  status: 'PENDING' | 'APPROVED' | 'REJECTED' | 'EXPIRED';
  risk_level: RiskLevel;
  decided_by?: string;
  decision_notes?: string;
  decided_at?: string;
  created_at: string;
}

export interface AuditEvent {
  id: number;
  user_id?: string;
  username?: string;
  agent_id?: string;
  task_id?: string;
  event_type: string;
  action?: string;
  details: Record<string, any>;
  ip_address?: string;
  created_at: string;
}

export interface Runbook {
  id: string;
  slug: string;
  title: string;
  description: string;
  created_from_task?: string;
  latest_version: number;
  variables: any[];
  steps: any[];
  created_at: string;
  updated_at: string;
}
