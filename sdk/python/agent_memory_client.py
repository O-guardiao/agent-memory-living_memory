import requests


class AgentMemoryClient:
    def __init__(self, base_url: str = "http://localhost:8080"):
        self.base_url = base_url.rstrip("/")

    def ingest_event(self, **payload):
        return self._post("/v1/events", payload)

    def search(self, **payload):
        return self._post("/v1/memories/search", payload)

    def assemble_context(self, **payload):
        return self._post("/v1/context/assemble", payload)

    def agentic_skills(self):
        r = requests.get(f"{self.base_url}/v1/agentic/skills", timeout=30)
        r.raise_for_status()
        return r.json()

    def agentic_decide(self, **payload):
        return self._post("/v1/agentic/decide", payload)

    def create_spec(self, **payload):
        return self._post("/v1/agentic/specs", payload)

    def create_plan(self, **payload):
        return self._post("/v1/agentic/plans", payload)

    def assemble_agentic_context(self, **payload):
        return self._post("/v1/agentic/context", payload)

    def _post(self, path: str, payload: dict):
        r = requests.post(f"{self.base_url}{path}", json=payload, timeout=30)
        r.raise_for_status()
        return r.json()
