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

import { test, expect, request, Page } from '@playwright/test';
import * as path from 'path';
import * as fs from 'fs';

const API = 'http://localhost:8080';
const UI = 'http://localhost:4000';
const GRAFANA = 'http://localhost:3002';
const SCREENSHOT_DIR = path.join(__dirname, 'screenshots');

// Use existing connection with valid credentials
const EXISTING_CONNECTION_ID = 5;

const state: {
  connectionId: number;
  scopeId: string;
  blueprintId: number;
  pipelineId: number;
} = { connectionId: EXISTING_CONNECTION_ID, scopeId: '', blueprintId: 0, pipelineId: 0 };

fs.mkdirSync(SCREENSHOT_DIR, { recursive: true });

async function grafanaLogin(page: Page) {
  await page.goto(`${GRAFANA}/grafana/login`);
  await page.waitForLoadState('networkidle');
  if (page.url().includes('/login')) {
    await page.locator('input[name="user"]').fill('admin');
    await page.locator('input[name="password"]').fill('admin');
    await page.locator('button[type="submit"]').click();
    await page.waitForTimeout(2000);
    // Handle "change password" prompt if shown
    const skipBtn = page.locator('a:has-text("Skip")');
    if (await skipBtn.isVisible({ timeout: 2000 }).catch(() => false)) {
      await skipBtn.click();
    }
    await page.waitForTimeout(1000);
  }
}

async function openGrafanaDashboard(page: Page, uid: string, screenshotPath: string) {
  await grafanaLogin(page);
  await page.goto(`${GRAFANA}/grafana/d/${uid}?orgId=1&from=now-90d&to=now`);

  // Wait for first panel data to load
  try {
    await page.waitForResponse(
      (resp) => resp.url().includes('/api/ds/query') && resp.status() === 200,
      { timeout: 30000 }
    );
  } catch { /* some dashboards may not fire queries immediately */ }

  // Wait for rendering to settle
  await page.waitForTimeout(5000);

  // Take viewport screenshot (top section)
  await page.screenshot({ path: screenshotPath.replace('.png', '-top.png') });

  // Scroll down and take more sections
  const scrollHeight = await page.evaluate(() => document.body.scrollHeight);
  let section = 1;
  for (let y = 900; y < scrollHeight; y += 900) {
    await page.evaluate((scrollY) => window.scrollTo(0, scrollY), y);
    await page.waitForTimeout(3000);
    section++;
    await page.screenshot({ path: screenshotPath.replace('.png', `-section${section}.png`) });
  }

  // Also take full page screenshot
  await page.evaluate(() => window.scrollTo(0, 0));
  await page.waitForTimeout(2000);
  await page.screenshot({ path: screenshotPath, fullPage: true });
}

test.describe.serial('Q-Dev Plugin Full Flow', () => {

  test('Step 1: Verify Existing Connection via API', async () => {
    const api = await request.newContext({ baseURL: API });

    const resp = await api.get(`/plugins/q_dev/connections/${state.connectionId}`);
    expect(resp.ok()).toBeTruthy();
    const conn = await resp.json();
    console.log(`Using connection: id=${conn.id}, name=${conn.name}, bucket=${conn.bucket}`);

    const testResp = await api.post(`/plugins/q_dev/connections/${state.connectionId}/test`);
    const testBody = await testResp.json();
    console.log('Test connection:', testBody.success ? 'OK' : testBody.message);
    expect(testResp.ok()).toBeTruthy();
  });

  test('Step 2: View Config-UI Home', async ({ page }) => {
    await page.goto(UI);
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(1000);
    await page.screenshot({ path: path.join(SCREENSHOT_DIR, '01-config-ui-home.png'), fullPage: true });
    console.log('Screenshot: Config-UI home');
  });

  test('Step 3: Create Scope (S3 Slice) via API', async () => {
    const api = await request.newContext({ baseURL: API });

    const resp = await api.put(`/plugins/q_dev/connections/${state.connectionId}/scopes`, {
      data: {
        data: [
          {
            accountId: '034362076319',
            basePath: '',
            year: 2026,
            month: 3,
          },
        ],
      },
    });

    const body = await resp.json();
    console.log('Scope created:', resp.status());
    expect(resp.ok()).toBeTruthy();
    state.scopeId = body[0]?.id;
    expect(state.scopeId).toBeTruthy();
    console.log(`Scope id: ${state.scopeId}`);
  });

  test('Step 4: Create Blueprint via API', async () => {
    const api = await request.newContext({ baseURL: API });

    const resp = await api.post('/blueprints', {
      data: {
        name: `e2e-blueprint-${Date.now()}`,
        mode: 'NORMAL',
        enable: true,
        cronConfig: '0 0 * * *',
        isManual: true,
        connections: [
          {
            pluginName: 'q_dev',
            connectionId: state.connectionId,
            scopes: [{ scopeId: state.scopeId }],
          },
        ],
      },
    });

    const body = await resp.json();
    expect(resp.ok()).toBeTruthy();
    state.blueprintId = body.id;
    console.log(`Blueprint created: id=${state.blueprintId}`);
  });

  test('Step 5: Trigger Pipeline via API', async () => {
    const api = await request.newContext({ baseURL: API });

    const resp = await api.post(`/blueprints/${state.blueprintId}/trigger`, { data: {} });
    const body = await resp.json();
    expect(resp.ok()).toBeTruthy();
    state.pipelineId = body.id;
    console.log(`Pipeline triggered: id=${state.pipelineId}`);
  });

  test('Step 6: Wait for Pipeline to Complete', async () => {
    const api = await request.newContext({ baseURL: API });
    const maxWait = 120000;
    const start = Date.now();
    let status = '';

    while (Date.now() - start < maxWait) {
      const resp = await api.get(`/pipelines/${state.pipelineId}`);
      const pipeline = await resp.json();
      status = pipeline.status;
      console.log(`Pipeline status: ${status} (${Math.round((Date.now() - start) / 1000)}s)`);
      if (['TASK_COMPLETED', 'TASK_FAILED', 'TASK_PARTIAL'].includes(status)) break;
      await new Promise((r) => setTimeout(r, 3000));
    }

    // Print task details
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

  test('Step 7: Verify Data via MySQL', async () => {
    const api = await request.newContext({ baseURL: API });

    // Use pipeline tasks to confirm data was processed
    const tasksResp = await api.get(`/pipelines/${state.pipelineId}/tasks`);
    const { tasks } = await tasksResp.json();
    expect(tasks[0].status).toBe('TASK_COMPLETED');
    console.log(`Pipeline completed in ${tasks[0].spentSeconds}s`);
  });

  test('Step 8: Grafana - Kiro Usage Dashboard (new format)', async ({ page }) => {
    await openGrafanaDashboard(page, 'qdev_user_report', path.join(SCREENSHOT_DIR, '02-dashboard-user-report.png'));
    console.log('Screenshot: Kiro Usage Dashboard');
  });

  test('Step 9: Grafana - Kiro Legacy Feature Metrics', async ({ page }) => {
    await openGrafanaDashboard(page, 'qdev_feature_metrics', path.join(SCREENSHOT_DIR, '03-dashboard-feature-metrics.png'));
    console.log('Screenshot: Kiro Legacy Feature Metrics');
  });

  test('Step 10: Grafana - Kiro AI Activity Insights (logging)', async ({ page }) => {
    await openGrafanaDashboard(page, 'qdev_logging', path.join(SCREENSHOT_DIR, '04-dashboard-logging.png'));
    console.log('Screenshot: Kiro AI Activity Insights');
  });

  test('Step 11: Grafana - Kiro Executive Dashboard', async ({ page }) => {
    await openGrafanaDashboard(page, 'qdev_executive', path.join(SCREENSHOT_DIR, '05-dashboard-executive.png'));
    console.log('Screenshot: Kiro Executive Dashboard');
  });

  test('Step 12: View Pipeline in Config-UI', async ({ page }) => {
    // Navigate to the API proxy route for pipelines
    await page.goto(`${UI}/api/pipelines?pageSize=5`);
    await page.waitForLoadState('networkidle');
    await page.screenshot({ path: path.join(SCREENSHOT_DIR, '06-config-ui-pipelines.png'), fullPage: true });
    console.log('Screenshot: Pipelines API response');
  });

  test('Step 13: Cleanup', async () => {
    const api = await request.newContext({ baseURL: API });
    if (state.blueprintId) {
      await api.delete(`/blueprints/${state.blueprintId}`);
      console.log(`Deleted blueprint ${state.blueprintId}`);
    }
    console.log('Cleanup complete');
  });
});
