import json
from typing import List, Optional
from ..models.schemas import HostContext, PlanRequest, ReplanRequest, DiagnosisRequest

def build_planning_prompt(req: PlanRequest) -> str:
    obs_text = ""
    if req.untrusted_observations:
        obs_text = "\n<UNTRUSTED_OBSERVATION>\n" + "\n---\n".join(req.untrusted_observations) + "\n</UNTRUSTED_OBSERVATION>\n"

    return f"""### USER INTENT:
"{req.intent}"

### TARGET HOST CONTEXT:
Hostname: {req.host_context.hostname}
OS: {req.host_context.os} ({req.host_context.distribution} {req.host_context.version})
Architecture: {req.host_context.architecture}
Reported Capabilities: {', '.join(req.host_context.capabilities)}
Currently Open Ports: {req.host_context.open_ports or []}
Active Services: {req.host_context.active_services or []}
{obs_text}
### SUPPORTED TYPED TOOLS:
{json.dumps(req.supported_tools, indent=2)}

### INSTRUCTION:
Generate a complete, minimal, safe, and verifiable step-by-step execution plan to accomplish the user intent on this host.

### EXAMPLE PLAN FOR NGINX ON PORT 8080:
{{
  "goal": "Install nginx and expose it on port 8080",
  "reasoning": "Install nginx package, configure default site to listen on port 8080, restart nginx systemd service, and verify service status, port 8080, and HTTP 200 response.",
  "steps": [
    {{
      "id": "step-1",
      "action": "install_package",
      "arguments": {{"name": "nginx"}},
      "reason": "Install nginx web server",
      "suggested_risk": "MEDIUM",
      "verification_strategy": {{"check_type": "package_installed", "target": "nginx", "expected": "installed"}}
    }},
    {{
      "id": "step-2",
      "action": "write_config_file",
      "arguments": {{
        "path": "/etc/nginx/sites-available/default",
        "content": "server {{\n    listen 8080 default_server;\n    listen [::]:8080 default_server;\n    root /var/www/html;\n    index index.html index.nginx-debian.html;\n    server_name _;\n    location / {{\n        try_files $uri $uri/ =404;\n    }}\n}}\n"
      }},
      "reason": "Configure nginx to listen on port 8080",
      "suggested_risk": "MEDIUM",
      "verification_strategy": null
    }},
    {{
      "id": "step-3",
      "action": "restart_service",
      "arguments": {{"name": "nginx"}},
      "reason": "Apply configuration changes and start nginx",
      "suggested_risk": "MEDIUM",
      "verification_strategy": {{"check_type": "systemd_active", "target": "nginx", "expected": "active"}}
    }}
  ],
  "overall_verification": [
    {{"check_type": "systemd_active", "target": "nginx", "expected": "active"}},
    {{"check_type": "tcp_port_open", "target": "8080", "expected": "open"}},
    {{"check_type": "http_probe", "target": "http://127.0.0.1:8080", "expected": "200"}}
  ]
}}

Respond ONLY with a JSON object conforming to the schema above:
"""

def build_replanning_prompt(req: ReplanRequest) -> str:
    prior_steps = [s.model_dump() if hasattr(s, "model_dump") else s for s in (req.prior_successful_steps or [])]
    prior_steps_json = json.dumps(prior_steps, indent=2)
    return f"""### REPLANNING REQUEST
A previous step in the execution plan failed.

Goal Intent: "{req.intent}"
Failed Step ID: {req.failed_step_id}
Failed Action: {req.failed_action}
Exit Code: {req.exit_code}

Prior Successful Steps:
{prior_steps_json}

<UNTRUSTED_OBSERVATION>
--- STDOUT ---
{req.untrusted_stdout}
--- STDERR ---
{req.untrusted_stderr}
</UNTRUSTED_OBSERVATION>

CRITICAL RECOVERY CONSTRAINTS:
1. INTENT INTEGRITY: Never silently change the user intent. If the user requested port 8080 and a port conflict occurred ("Address already in use"), DO NOT propose port 8081 or any other port. Propose diagnostic inspection or state that operator intervention is required.
2. DO NOT propose destructive commands (kill -9, rm -rf) against unknown running processes.

Generate a revised recovery plan or diagnostic steps to address the condition.
Respond ONLY with a valid JSON object strictly matching this schema:
{{
  "goal": "{req.intent}",
  "reasoning": "Explanation of recovery approach or why operator decision is required",
  "steps": [
    {{
      "id": "step-1",
      "action": "execute_command",
      "arguments": {{"command": "ss -tulpn | grep 8080"}},
      "reason": "Inspect which process is occupying the requested port without modifying system state",
      "suggested_risk": "READ_ONLY",
      "verification_strategy": null
    }}
  ],
  "overall_verification": []
}}
"""

def build_diagnosis_prompt(req: DiagnosisRequest) -> str:
    return f"""### DIAGNOSIS REQUEST
Target Host: {req.host_context.hostname} ({req.host_context.distribution} {req.host_context.version})
Reported Symptom: "{req.symptom}"

<UNTRUSTED_OBSERVATION>
{req.untrusted_logs}
</UNTRUSTED_OBSERVATION>

Analyze the untrusted logs and symptoms. Provide a structured diagnosis and suggested remediation steps.
The "root_cause" MUST be selected from one of these canonical categories:
- PORT_CONFLICT: "Address already in use", port occupied, bind failure
- INVALID_CONFIG: "syntax error", directive not allowed, configuration test failed, nginx -t error
- PACKAGE_MISSING: package not installed, dpkg error, command not found for standard package
- SERVICE_STOPPED: systemd service is inactive (dead), stopped, or failed to start
- SERVICE_CRASH: service crashed, core dumped, terminated by fatal signal (SIGSEGV)
- RESTART_LOOP: unit continuously restarting, start-limit-hit
- PERMISSION_DENIED: permission denied, operation not permitted, read-only filesystem
- DNS_FAILURE: name or service not known, NXDOMAIN, host resolution failure
- CONNECTION_REFUSED: connection refused on target port, service daemon not listening
- HTTP_APPLICATION_FAILURE: HTTP 500/502/503 status code response from application
- DISK_PRESSURE: no space left on device, filesystem full, inode exhaustion
- CONTAINER_CRASH: docker container exited unexpectedly or OOMKilled
- CONTAINER_RESTART_LOOP: container crash looping
- CONTAINER_UNHEALTHY: container healthcheck failing
- IMAGE_NOT_FOUND: docker pull image not found or repository unavailable
- AGENT_DISCONNECTED: agent transport closed, heartbeat timeout
- TIMEOUT: execution deadline exceeded, verification timeout
- UNSUPPORTED_RESOURCE: unit could not be found, missing-unit, non-existent service/resource
- POLICY_DENIED: action forbidden by policy or RBAC security guardrail
- IDEMPOTENT_SATISFIED: target condition already met, nothing to do
- NONE: no error or anomaly detected
- UNKNOWN: root cause cannot be determined from available logs

Respond ONLY with a JSON object conforming to:
{{
  "identified_problem": "Summary of the fault",
  "root_cause": "PORT_CONFLICT",
  "confidence": 0.95,
  "evidence": [
    {{
      "source": "evidence_log_analysis",
      "detail": "Observed port conflict: address already in use"
    }}
  ],
  "remediation_steps": [
    {{
      "id": "step-1",
      "action": "execute_command",
      "arguments": {{"command": "ss -tulpn"}},
      "reason": "Identify process occupying port",
      "suggested_risk": "READ_ONLY",
      "verification_strategy": null
    }}
  ]
}}
"""

