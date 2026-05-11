import http from 'k6/http';
import { check } from 'k6';

export default function () {
  const res = http.post('http://localhost:8080/v1/events', JSON.stringify({
    tenant_id: 'tenant_demo', user_id: 'user_123', session_id: 'load', role: 'user', content: 'Eu prefiro Go para serviços de memória.'
  }), { headers: { 'Content-Type': 'application/json' } });
  check(res, { 'status is 201': (r) => r.status === 201 });
}
