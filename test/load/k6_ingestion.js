import http from 'k6/http';
import { check } from 'k6';

const baseURL = __ENV.MEMORY_BASE_URL || 'http://localhost:8080';

export const options = {
  vus: Number(__ENV.K6_VUS || 10),
  duration: __ENV.K6_DURATION || '30s',
  thresholds: {
    http_req_failed: ['rate<0.01'],
    http_req_duration: ['p(95)<400'],
    checks: ['rate>0.99'],
  },
  summaryTrendStats: ['min', 'avg', 'med', 'p(90)', 'p(95)', 'p(99)', 'max'],
};

export default function () {
  const tenant = `tenant_${__VU % 8}`;
  const user = `user_${__VU % 128}`;
  const res = http.post(`${baseURL}/v1/events`, JSON.stringify({
    tenant_id: tenant, user_id: user, session_id: 'load', role: 'user',
    content: `Eu prefiro Go para serviços de memória. (vu ${__VU} iter ${__ITER})`,
  }), { headers: { 'Content-Type': 'application/json' } });
  check(res, { 'status is 201 or 202': (r) => r.status === 201 || r.status === 202 });
}
