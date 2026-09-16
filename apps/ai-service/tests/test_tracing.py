from fastapi.testclient import TestClient
from app.main import app

client = TestClient(app)

def test_tracing_middleware_extracts_traceparent():
    trace_id = "4bf92f3577b34da6a3ce929d0e0e4736"
    span_id = "00f067aa0ba902b7"
    traceparent = f"00-{trace_id}-{span_id}-01"

    response = client.get("/health", headers={"traceparent": traceparent})
    assert response.status_code == 200
    assert response.headers.get("traceparent") == traceparent
    assert response.headers.get("x-trace-id") == trace_id

def test_tracing_middleware_extracts_xtraceid():
    custom_trace_id = "custom-trace-1234567890abcdef"
    response = client.get("/health", headers={"x-trace-id": custom_trace_id})
    assert response.status_code == 200
    assert response.headers.get("x-trace-id") == custom_trace_id
