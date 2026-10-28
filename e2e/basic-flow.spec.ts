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
const UI = 'http://localhost:4000';

const state: {
  connectionId: number;
  projectName: string;
} = { connectionId: 0, projectName: '' };

test.describe.serial('Basic Flow (no external credentials)', () => {
  test('Set onboard flag and verify home page', async ({ page }) => {
    const api = await request.newContext({ baseURL: API });
    await api.put('/store/onboard', {
      data: { step: 4, records: [], done: true, projectName: '', plugin: '' },
    });

    await page.goto(UI);
    await page.waitForLoadState('networkidle');
    await expect(page).toHaveTitle(/DevLake/);
  });

  test('Create webhook connection via API', async ({ page }) => {
    const api = await request.newContext({ baseURL: API });

    const resp = await api.post('/plugins/webhook/connections', {
      data: { name: `e2e-webhook-${Date.now()}` },
    });
    if (!resp.ok()) {
      const text = await resp.text();
      throw new Error(`Webhook creation failed (${resp.status()}): ${text}`);
    }
    const body = await resp.json();
    state.connectionId = body.id;
    expect(state.connectionId).toBeGreaterThan(0);

    await page.goto(`${UI}/connections`);
    await page.waitForLoadState('networkidle');
  });

  test('Create project via API', async ({ page }) => {
    const api = await request.newContext({ baseURL: API });
    state.projectName = `e2e-project-${Date.now()}`;

    const resp = await api.post('/projects', {
      data: {
        name: state.projectName,
        description: 'E2E test project',
      },
    });
    if (!resp.ok()) {
      const text = await resp.text();
      throw new Error(`Project creation failed (${resp.status()}): ${text}`);
    }

    await page.goto(UI);
    await page.waitForLoadState('networkidle');
  });

  test('Project visible in UI', async ({ page }) => {
    await page.goto(`${UI}/projects`);
    await page.waitForLoadState('networkidle');

    const projectLink = page.locator(`text=${state.projectName}`);
    await expect(projectLink).toBeVisible({ timeout: 15000 });
  });

  test('Connections page loads', async ({ page }) => {
    await page.goto(`${UI}/connections`);
    await page.waitForLoadState('networkidle');
    await expect(page.getByRole('heading', { name: 'Webhooks', exact: true })).toBeVisible({ timeout: 10000 });
  });

  test('Pipelines page loads', async ({ page }) => {
    await page.goto(`${UI}/pipelines`);
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(1000);
    await expect(page.locator('text=Error')).not.toBeVisible();
  });

  test('Cleanup', async ({ page }) => {
    const api = await request.newContext({ baseURL: API });

    if (state.projectName) {
      await api.delete(`/projects/${state.projectName}`);
    }
    if (state.connectionId) {
      await api.delete(`/plugins/webhook/connections/${state.connectionId}`);
    }

    await page.goto(UI);
    await page.waitForLoadState('networkidle');
  });
});
