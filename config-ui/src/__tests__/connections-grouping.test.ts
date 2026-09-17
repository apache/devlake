/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

import { describe, it, expect } from 'vitest';

import { splitPluginsByInitial } from '@/routes/connection/connections';

// The plugin list arrives ordered by each config's `sort` value, which is not
// alphabetical: plugins have been appended in the order they were added.
const byName: Record<string, string> = {
  argocd: 'ArgoCD',
  clickup: 'ClickUp',
  opsgenie: 'Opsgenie',
  pagerduty: 'PagerDuty',
  asana: 'Asana',
  linear: 'Linear',
  zentao: 'ZenTao',
  incidentio: 'incident.io',
  azuredevops: 'Azure DevOps',
};
const nameOf = (plugin: string) => byName[plugin] ?? plugin;

describe('splitPluginsByInitial', () => {
  it('groups by the displayed name, not by list position', () => {
    const [an, oz] = splitPluginsByInitial(
      ['argocd', 'clickup', 'opsgenie', 'pagerduty', 'asana', 'linear', 'zentao'],
      nameOf,
    );
    // asana and linear follow opsgenie in the list, and used to be filed O-Z.
    expect(an).toEqual(['argocd', 'clickup', 'asana', 'linear']);
    expect(oz).toEqual(['opsgenie', 'pagerduty', 'zentao']);
  });

  it('keeps every plugin, wherever it sits in the list', () => {
    const plugins = Object.keys(byName);
    const [an, oz] = splitPluginsByInitial(plugins, nameOf);
    expect([...an, ...oz].sort()).toEqual([...plugins].sort());
  });

  it('compares case-insensitively, so a lowercase name still groups correctly', () => {
    const [an, oz] = splitPluginsByInitial(['incidentio', 'zentao'], nameOf);
    expect(an).toEqual(['incidentio']);
    expect(oz).toEqual(['zentao']);
  });

  it('falls back to the plugin id when a config has no name', () => {
    const [an, oz] = splitPluginsByInitial(['unknown-plugin', 'another'], (p) => p);
    expect(an).toEqual(['another']);
    expect(oz).toEqual(['unknown-plugin']);
  });
});
