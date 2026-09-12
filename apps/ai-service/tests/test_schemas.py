import pytest
from app.models.schemas import PlanResponse, StepPlan, RiskLevel, VerificationRequirement

def test_plan_response_schema():
    data = {
        "goal": "Test Nginx Goal",
        "reasoning": "Install nginx package and verify port",
        "steps": [
            {
                "id": "step-1",
                "action": "install_package",
                "arguments": {"name": "nginx"},
                "reason": "Install web server",
                "suggested_risk": "MEDIUM",
                "verification_strategy": {
                    "check_type": "package_installed",
                    "target": "nginx",
                    "expected": "installed",
                    "timeout_sec": 30
                }
            }
        ],
        "overall_verification": [
            {
                "check_type": "systemd_active",
                "target": "nginx",
                "expected": "active",
                "timeout_sec": 10
            }
        ]
    }
    plan = PlanResponse.model_validate(data)
    assert plan.goal == "Test Nginx Goal"
    assert len(plan.steps) == 1
    assert plan.steps[0].action == "install_package"
    assert plan.steps[0].suggested_risk == RiskLevel.MEDIUM
    assert plan.steps[0].verification_strategy.target == "nginx"
