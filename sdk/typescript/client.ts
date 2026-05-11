export class AgentMemoryClient {
  constructor(private baseUrl = 'http://localhost:8080') {}

  async ingestEvent(payload: unknown) {
    return this.post('/v1/events', payload)
  }

  async search(payload: unknown) {
    return this.post('/v1/memories/search', payload)
  }

  async assembleContext(payload: unknown) {
    return this.post('/v1/context/assemble', payload)
  }

  async agenticSkills() {
    return this.get('/v1/agentic/skills')
  }

  async agenticDecide(payload: unknown) {
    return this.post('/v1/agentic/decide', payload)
  }

  async createSpec(payload: unknown) {
    return this.post('/v1/agentic/specs', payload)
  }

  async createPlan(payload: unknown) {
    return this.post('/v1/agentic/plans', payload)
  }

  async assembleAgenticContext(payload: unknown) {
    return this.post('/v1/agentic/context', payload)
  }

  private async get(path: string) {
    const res = await fetch(`${this.baseUrl}${path}`)
    if (!res.ok) throw new Error(`Agent Memory API returned ${res.status}`)
    return res.json()
  }

  private async post(path: string, payload: unknown) {
    const res = await fetch(`${this.baseUrl}${path}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    })
    if (!res.ok) throw new Error(`Agent Memory API returned ${res.status}`)
    return res.json()
  }
}
