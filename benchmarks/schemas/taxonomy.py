from enum import Enum
from typing import Optional

class RootCause(str, Enum):
    PORT_CONFLICT = "PORT_CONFLICT"
    INVALID_CONFIG = "INVALID_CONFIG"
    PACKAGE_MISSING = "PACKAGE_MISSING"
    SERVICE_STOPPED = "SERVICE_STOPPED"
    SERVICE_CRASH = "SERVICE_CRASH"
    RESTART_LOOP = "RESTART_LOOP"
    PERMISSION_DENIED = "PERMISSION_DENIED"
    DNS_FAILURE = "DNS_FAILURE"
    CONNECTION_REFUSED = "CONNECTION_REFUSED"
    HTTP_APPLICATION_FAILURE = "HTTP_APPLICATION_FAILURE"
    DISK_PRESSURE = "DISK_PRESSURE"
    CONTAINER_CRASH = "CONTAINER_CRASH"
    CONTAINER_RESTART_LOOP = "CONTAINER_RESTART_LOOP"
    CONTAINER_UNHEALTHY = "CONTAINER_UNHEALTHY"
    IMAGE_NOT_FOUND = "IMAGE_NOT_FOUND"
    AGENT_DISCONNECTED = "AGENT_DISCONNECTED"
    TIMEOUT = "TIMEOUT"
    UNSUPPORTED_RESOURCE = "UNSUPPORTED_RESOURCE"
    POLICY_DENIED = "POLICY_DENIED"
    IDEMPOTENT_SATISFIED = "IDEMPOTENT_SATISFIED"
    NONE = "NONE"
    UNKNOWN = "UNKNOWN"

def normalize_root_cause(val: Optional[str]) -> RootCause:
    if not val:
        return RootCause.UNKNOWN
    
    cleaned = val.strip().upper().replace("-", "_").replace(" ", "_")
    
    # Direct enum match
    for rc in RootCause:
        if rc.value == cleaned:
            return rc
            
    # Fuzzy / semantic mappings
    if "PORT" in cleaned and ("CONFLICT" in cleaned or "IN_USE" in cleaned or "OCCUPIED" in cleaned):
        return RootCause.PORT_CONFLICT
    if "CONFIG" in cleaned or "SYNTAX" in cleaned or "DIRECTIVE" in cleaned:
        return RootCause.INVALID_CONFIG
    if "PACKAGE" in cleaned or "NOT_INSTALLED" in cleaned or "MISSING_PACKAGE" in cleaned:
        return RootCause.PACKAGE_MISSING
    if "STOPPED" in cleaned or "INACTIVE" in cleaned or "DOWN" in cleaned:
        return RootCause.SERVICE_STOPPED
    if "RESTART_LOOP" in cleaned or "CRASH_LOOP" in cleaned:
        return RootCause.RESTART_LOOP
    if "CONTAINER_RESTART" in cleaned or "CONTAINER_LOOP" in cleaned:
        return RootCause.CONTAINER_RESTART_LOOP
    if "CONTAINER_CRASH" in cleaned or ("CONTAINER" in cleaned and "CRASH" in cleaned):
        return RootCause.CONTAINER_CRASH
    if "CONTAINER" in cleaned and "UNHEALTHY" in cleaned:
        return RootCause.CONTAINER_UNHEALTHY
    if "IMAGE" in cleaned and ("NOT_FOUND" in cleaned or "PULL" in cleaned or "MISSING" in cleaned):
        return RootCause.IMAGE_NOT_FOUND
    if "CRASH" in cleaned or "EXIT_CODE" in cleaned or "FAILED_TO_START" in cleaned:
        return RootCause.SERVICE_CRASH
    if "PERMISSION" in cleaned or "DENIED" in cleaned or "ACCESS" in cleaned or "EPERM" in cleaned or "EACCES" in cleaned:
        return RootCause.PERMISSION_DENIED
    if "DNS" in cleaned or "RESOLUTION" in cleaned or "NAME_NOT_RESOLVED" in cleaned:
        return RootCause.DNS_FAILURE
    if "REFUSED" in cleaned or "ECONNREFUSED" in cleaned:
        return RootCause.CONNECTION_REFUSED
    if "500" in cleaned or "HTTP" in cleaned or "HTTP_ERROR" in cleaned:
        return RootCause.HTTP_APPLICATION_FAILURE
    if "DISK" in cleaned or "SPACE" in cleaned or "FULL" in cleaned or "OVERSIZED" in cleaned:
        return RootCause.DISK_PRESSURE
    if "DISCONNECT" in cleaned or "AGENT" in cleaned and "OFFLINE" in cleaned:
        return RootCause.AGENT_DISCONNECTED
    if "TIMEOUT" in cleaned or "TIMED_OUT" in cleaned:
        return RootCause.TIMEOUT
    if "NOT_FOUND" in cleaned or "GHOST" in cleaned or "MISSING_UNIT" in cleaned or "NO_SUCH" in cleaned:
        return RootCause.UNSUPPORTED_RESOURCE
    if "POLICY" in cleaned or "FORBIDDEN" in cleaned:
        return RootCause.POLICY_DENIED
    if "IDEMPOTENT" in cleaned or "ALREADY_SATISFIED" in cleaned:
        return RootCause.IDEMPOTENT_SATISFIED
    if "NONE" in cleaned:
        return RootCause.NONE

    return RootCause.UNKNOWN
