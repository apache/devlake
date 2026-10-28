/*
Licensed to the Apache Software Foundation (ASF) under one or more
contributor license agreements.  See the NOTICE file distributed with
this work for additional information regarding copyright ownership.
The ASF licenses this file to You under the Apache License, Version 2.0
(the "License"); you may not use this file except in compliance with
the License.  You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import { test, expect, request } from '@playwright/test';

const API = 'http://localhost:8080';
const TOKEN = process.env.CODECOV_TOKEN;
const ORG = process.env.CODECOV_ORG ?? 'konflux-ci';
const SERVICE = process.env.CODECOV_SERVICE ?? 'github';
const ENDPOINT = process.env.CODECOV_ENDPOINT ?? 'https://api.codecov.io';
const REPO = process.env.CODECOV_REPO ?? 'konflux-ci/coverport';

const EXPECTED_SUBTASKS = [
  'CollectFlags',
  'ConvertFlags',
  'CollectCommits',
  'ExtractCommits',
  'CollectCommitTotals',
  'CollectCommitCoverage',
  'CollectComparison',
  'CollectFlagCoverageTrend',
  'CollectRepoConfig',
  'ConvertComparison',
  'ConvertCoverage',
  'ConvertCommitCoverage',
  'ConvertCoverageTrend',
];

const state = {
  connectionId: 0,
  scopeConfigId: 0,
  scopeId: '',
  blueprintId: 0,
  pipelineId: 0,
  remoteRepo: null as Record<string, unknown> | null,
};

async function cleanupResources() {
  const api = await request.newContext({ baseURL: API });

  if (state.blueprintId) {
    await api.delete(`/blueprints/${state.blueprintId}`);
    console.log(`Deleted blueprint ${state.blueprintId}`);
  }
  if (state.scopeId) {
    await api.delete(
      `/plugins/codecov/connections/${state.connectionId}/scopes/${encodeURIComponent(state.scopeId)}`,
    );
    console.log(`Deleted scope ${state.scopeId}`);
  }
  if (state.scopeConfigId) {
    await api.delete(
      `/plugins/codecov/connections/${state.connectionId}/scope-configs/${state.scopeConfigId}`,
    );
    console.log(`Deleted scope config ${state.scopeConfigId}`);
  }
  if (state.connectionId) {
    await api.delete(`/plugins/codecov/connections/${state.connectionId}`);
    console.log(`Deleted connection ${state.connectionId}`);
  }
}

test.describe.serial('Codecov Plugin Full Flow', () => {
  test.skip(!process.env.CODECOV_TOKEN, 'CODECOV_TOKEN not set');
  test.setTimeout(900_000);

  test.afterAll(async () => {
    if (state.connectionId) {
      await cleanupResources();
      console.log('Cleanup complete');
    }
  });

  test('Step 1: Create Connection', async () => {
    const api = await request.newContext({ baseURL: API });
    const resp = await api.post('/plugins/codecov/connections', {
      data: {
        name: `e2e-codecov-${Date.now()}`,
        token: TOKEN,
        organization: ORG,
        service: SERVICE,
        endpoint: ENDPOINT,
      },
    });
    expect(resp.ok()).toBeTruthy();
    const conn = await resp.json();
    state.connectionId = conn.id;
    console.log(`Connection created: id=${state.connectionId}`);
  });

  test('Step 2: Test Connection', async () => {
    const api = await request.newContext({ baseURL: API });
    const resp = await api.post(`/plugins/codecov/connections/${state.connectionId}/test`);
    const body = await resp.json();
    expect(resp.ok()).toBeTruthy();
    expect(body.success).toBe(true);
    console.log('Test connection: OK');
  });

  test('Step 3: Discover Remote Scopes', async () => {
    const api = await request.newContext({ baseURL: API });
    const resp = await api.get(`/plugins/codecov/connections/${state.connectionId}/remote-scopes`);
    expect(resp.ok()).toBeTruthy();
    const body = await resp.json();

    const repo = body.children?.find((c: { fullName: string }) => c.fullName === REPO);
    expect(repo, `Repository ${REPO} not found in remote-scopes`).toBeTruthy();
    state.remoteRepo = repo.data;
    console.log(`Found remote repo: ${REPO}`);
  });

  test('Step 4: Create Scope Config', async () => {
    const api = await request.newContext({ baseURL: API });
    const resp = await api.post(
      `/plugins/codecov/connections/${state.connectionId}/scope-configs`,
      { data: { name: `e2e-scope-config-${Date.now()}`, entities: ['CODE'] } },
    );
    expect(resp.ok()).toBeTruthy();
    const body = await resp.json();
    state.scopeConfigId = body.id;
    console.log(`Scope config created: id=${state.scopeConfigId}`);
  });

  test('Step 5: Add Scope', async () => {
    const api = await request.newContext({ baseURL: API });
    const resp = await api.put(`/plugins/codecov/connections/${state.connectionId}/scopes`, {
      data: {
        data: [{
          ...state.remoteRepo,
          scopeConfigId: state.scopeConfigId,
        }],
      },
    });
    expect(resp.ok()).toBeTruthy();
    const scopes = await resp.json();
    state.scopeId = scopes[0].codecovId;
    expect(state.scopeId).toBe(REPO);
    console.log(`Scope added: id=${state.scopeId}`);
  });

  test('Step 6: Create Blueprint via API', async () => {
    const api = await request.newContext({ baseURL: API });
    const resp = await api.post('/blueprints', {
      data: {
        name: `e2e-blueprint-${Date.now()}`,
        mode: 'NORMAL',
        enable: true,
        cronConfig: '0 0 * * *',
        isManual: true,
        connections: [{
          pluginName: 'codecov',
          connectionId: state.connectionId,
          scopes: [{ scopeId: state.scopeId }],
        }],
      },
    });
    expect(resp.ok()).toBeTruthy();
    const body = await resp.json();
    state.blueprintId = body.id;
    console.log(`Blueprint created: id=${state.blueprintId}`);
  });

  test('Step 7: Trigger Pipeline via API', async () => {
    const api = await request.newContext({ baseURL: API });
    const resp = await api.post(`/blueprints/${state.blueprintId}/trigger`, { data: {} });
    expect(resp.ok()).toBeTruthy();
    const body = await resp.json();
    state.pipelineId = body.id;
    console.log(`Pipeline triggered: id=${state.pipelineId}`);
  });

  test('Step 8: Wait for Pipeline to Complete', async () => {
    const api = await request.newContext({ baseURL: API });
    const maxWait = 900_000; // 15 min — first Codecov collection can be slow
    const start = Date.now();
    let status = '';

    while (Date.now() - start < maxWait) {
      const resp = await api.get(`/pipelines/${state.pipelineId}`);
      const pipeline = await resp.json();
      status = pipeline.status;
      console.log(`Pipeline status: ${status} (${Math.round((Date.now() - start) / 1000)}s)`);
      if (['TASK_COMPLETED', 'TASK_FAILED', 'TASK_PARTIAL'].includes(status)) break;
      await new Promise((r) => setTimeout(r, 5000));
    }

    const tasksResp = await api.get(`/pipelines/${state.pipelineId}/tasks`);
    if (tasksResp.ok()) {
      const { tasks } = await tasksResp.json();
      for (const t of tasks || []) {
        console.log(`  Task ${t.id}: ${t.status}${t.failedSubTask ? ` (failed: ${t.failedSubTask})` : ''}`);
        if (t.message) console.log(`    Error: ${t.message.substring(0, 300)}`);
      }
    }

    expect(status).toBe('TASK_COMPLETED');
  });

  test('Step 9: Verify Pipeline Subtasks', async () => {
    const api = await request.newContext({ baseURL: API });

    const tasksResp = await api.get(`/pipelines/${state.pipelineId}/tasks`);
    expect(tasksResp.ok()).toBeTruthy();
    const { tasks } = await tasksResp.json();
    expect(tasks[0].status).toBe('TASK_COMPLETED');
    expect(tasks[0].subtasks).toEqual(EXPECTED_SUBTASKS);
    console.log(`Pipeline completed in ${tasks[0].spentSeconds}s with ${tasks[0].subtasks.length} subtasks`);
  });

  test('Step 10: Cleanup', async () => {
    await cleanupResources();
    state.connectionId = 0;
    console.log('Cleanup complete');
  });
});
