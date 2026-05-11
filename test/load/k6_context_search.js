import http from 'k6/http';
import { check } from 'k6';

export default function () {
  const res = http.post('http://localhost:8080/v1/context/assemble', JSON.stringify({
    tenant_id: 'tenant_demo', user_id: 'user_123', query: 'preferência de linguagem', limit: 5
  }), { headers: { 'Content-Type': 'application/json' } });
  check(res, { 'status is 200': (r) => r.status === 200 });
}
