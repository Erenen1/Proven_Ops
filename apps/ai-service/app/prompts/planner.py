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
Generate a minimal, safe, step-by-step execution plan to accomplish the user intent on this host.
Include deterministic verification strategies for any state-changing operations.
Respond ONLY with a JSON object conforming to the following structure:
{{
  "goal": "Brief description of the objective",
  "reasoning": "Technical rationale for the selected steps",
  "steps": [
    {{
      "id": "step-1",
      "action": "action_name",
      "arguments": {{ "arg_name": "value" }},
      "reason": "Why this action is performed",
      "suggested_risk": "READ_ONLY | LOW | MEDIUM | HIGH",
      "verification_strategy": {{
        "check_type": "systemd_active | tcp_port_open | http_probe | package_installed",
        "target": "service_name | port_number | url",
        "expected": "expected value or condition"
      }}
    }}
  ],
  "overall_verification": [
    {{
      "check_type": "http_probe | tcp_port_open | systemd_active",
      "target": "target identifier",
      "expected": "expected condition"
    }}
  ]
}}
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
