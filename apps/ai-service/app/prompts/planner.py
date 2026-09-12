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
      "verification_strategy": {{"check_type": "systemd_active", "target": "nginx", "expected": "active"}}
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
    prior_steps_json = json.dumps([s.model_dump() for s in req.prior_successful_steps], indent=2)
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

Generate a revised recovery plan or diagnostic steps to fix the condition and proceed toward the goal.
Respond ONLY with the JSON plan object structure.
"""

def build_diagnosis_prompt(req: DiagnosisRequest) -> str:
    return f"""### DIAGNOSIS REQUEST
Target Host: {req.host_context.hostname} ({req.host_context.distribution} {req.host_context.version})
Reported Symptom: "{req.symptom}"

<UNTRUSTED_OBSERVATION>
{req.untrusted_logs}
</UNTRUSTED_OBSERVATION>

Analyze the untrusted logs and symptoms. Provide a structured diagnosis and suggested remediation steps.
Respond ONLY with a JSON object conforming to:
{{
  "identified_problem": "Summary of the fault",
  "root_cause": "Detailed technical root cause",
  "confidence": 0.95,
  "remediation_steps": [
    {{
      "id": "step-1",
      "action": "action_name",
      "arguments": {{}},
      "reason": "Remediation rationale",
      "suggested_risk": "MEDIUM",
      "verification_strategy": null
    }}
  ]
}}
"""
