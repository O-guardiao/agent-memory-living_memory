import http from 'k6/http';
import { check } from 'k6';

const profile = __ENV.MEMORY_SCALE_PROFILE || '100k';
const baseURL = __ENV.MEMORY_BASE_URL || 'http://localhost:8080';

const profiles = {
  '100k': { vus: 25, duration: '3m', p95: 250, ingestRate: 100000 },
  '1m': { vus: 75, duration: '5m', p95: 400, ingestRate: 1000000 },
  '10m': { vus: 150, duration: '10m', p95: 750, ingestRate: 10000000 },
};

const selected = profiles[profile] || profiles['100k'];

export const options = {
  vus: selected.vus,
  duration: selected.duration,
  thresholds: {
    http_req_failed: ['rate<0.01'],
    http_req_duration: [`p(95)<${selected.p95}`],
    checks: ['rate>0.99'],
  },
  summaryTrendStats: ['min', 'avg', 'med', 'p(90)', 'p(95)', 'p(99)', 'max'],
};

export default function () {
  const tenant = `tenant_${__VU % 16}`;
  const user = `user_${__VU % 512}`;
  const project = `project_${__VU % 32}`;
  const sequence = `${profile}_${__VU}_${__ITER}`;

  const ingest = http.post(
    `${baseURL}/v1/events`,
    JSON.stringify({
      tenant_id: tenant,
      user_id: user,
      project_id: project,
      session_id: `scale_${profile}`,
      role: 'user',
      content: `Scale profile ${profile} synthetic memory ${sequence}. User prefers governed MemoryOps and token ROI checks.`,
      metadata: {
        scale_profile: profile,
        target_records: selected.ingestRate,
      },
    }),
    { headers: { 'Content-Type': 'application/json' } },
  );
  check(ingest, { 'ingest status is 201': (r) => r.status === 201 });

  const context = http.post(
    `${baseURL}/v1/context/assemble`,
    JSON.stringify({
      tenant_id: tenant,
      user_id: user,
      project_id: project,
      query: 'governed MemoryOps token ROI checks',
      limit: 8,
      filters: { status: 'active' },
    }),
    { headers: { 'Content-Type': 'application/json' } },
  );
  check(context, {
    'context status is 200': (r) => r.status === 200,
    'context has no scope leak marker': (r) => !r.body.includes('tenant_foreign'),
  });
}
