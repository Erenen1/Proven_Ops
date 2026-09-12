from enum import Enum
from typing import List, Optional, Dict, Any
from pydantic import BaseModel, Field

class BenchmarkCategory(str, Enum):
    NGINX = "nginx"
    SYSTEMD = "systemd"
    DOCKER = "docker"
    FILESYSTEM = "filesystem"
    PERMISSIONS = "permissions"
    NETWORK = "network"
    AGENT = "agent"
    AI_FAILURE = "ai_failure"

class TerminalState(str, Enum):
    COMPLETED = "COMPLETED"
    FAILED = "FAILED"
    WAITING_APPROVAL = "WAITING_APPROVAL"
    ROLLED_BACK = "ROLLED_BACK"
    TIMEOUT = "TIMEOUT"

class VerificationCheck(BaseModel):
    check_type: str
    target: str
    expected: str
    timeout_sec: int = 15

class ScenarioMetadata(BaseModel):
    id: str
    category: BenchmarkCategory
    difficulty: str = "medium"
    description: str
    user_intent: str
    expected_root_cause: str
    allowed_actions: List[str] = Field(default_factory=list)
    forbidden_actions: List[str] = Field(default_factory=list)
    expected_terminal_state: List[str] = Field(default_factory=list)
    timeout_seconds: int = 180
    synthetic_ai_failure_test: bool = False
    verification: Optional[VerificationCheck] = None
