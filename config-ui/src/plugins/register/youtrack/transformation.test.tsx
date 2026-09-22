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

import { useState } from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { ThemeProvider as StyledThemeProvider } from 'styled-components';

import { getTheme } from '@/theme/tokens';

import { YoutrackTransformation } from './transformation';

// jsdom has no matchMedia, which antd's responsive observer requires
window.matchMedia =
  window.matchMedia ||
  ((query: string) =>
    ({
      matches: false,
      media: query,
      onchange: null,
      addListener: () => {},
      removeListener: () => {},
      addEventListener: () => {},
      removeEventListener: () => {},
      dispatchEvent: () => false,
    }) as MediaQueryList);

// jsdom has no ResizeObserver, which rc-components (Table, AutoComplete) use
class ResizeObserverStub {
  observe() {}
  unobserve() {}
  disconnect() {}
}
window.ResizeObserver = window.ResizeObserver || (ResizeObserverStub as any);

const scopeConfigProjects = vi.fn();
const customFields = vi.fn();

vi.mock('@/api', () => ({
  default: {
    plugin: {
      youtrack: {
        scopeConfigProjects: (...args: any[]) => scopeConfigProjects(...args),
        customFields: (...args: any[]) => customFields(...args),
      },
    },
  },
}));

// one ProjectCustomField entry of the proxy response
const cf = (
  shortName: string,
  name: string,
  valueType: string,
  isMultiValue: boolean,
  values?: Array<{ name: string; isResolved?: boolean; ordinal: number }>,
) => ({
  id: `cf-${shortName}-${name}`,
  $type: 'XProjectCustomField',
  field: {
    id: `f-${name}`,
    name,
    fieldType: { id: `ft-${valueType}`, valueType, isMultiValue },
  },
  bundle: values
    ? {
        id: `b-${shortName}-${name}`,
        values: values.map((v, i) => ({ id: `v-${i}`, isResolved: false, archived: false, ...v })),
      }
    : null,
  project: { shortName },
});

const cbStates = [
  { name: 'Backlog', ordinal: 0 },
  { name: 'Declined', ordinal: 1, isResolved: true },
  ...Array.from({ length: 31 }, (_, i) => ({ name: `State ${i + 2}`, ordinal: i + 2 })),
];

const FIELDS: Record<string, any[]> = {
  '0-1': [
    cf('PROJ2', 'Client type', 'enum', false, [
      { name: 'Bug', ordinal: 0 },
      { name: 'Story', ordinal: 1 },
      { name: 'Subtask', ordinal: 2 },
    ]),
    cf('PROJ2', 'State', 'state', false, [
      { name: 'Backlog', ordinal: 0 },
      { name: 'Closed', ordinal: 1, isResolved: true },
    ]),
    cf('PROJ2', 'Assignee', 'user', false),
    cf('PROJ2', 'Story points', 'integer', false),
    // multi-value fields are ineligible for every slot and every union
    cf('PROJ2', 'Tags', 'enum', true, [{ name: 'tag-x', ordinal: 0 }]),
  ],
  '0-2': [
    cf('PROJ1', 'Client type', 'enum', false, [
      { name: 'Bug', ordinal: 0 },
      { name: 'Sub-task', ordinal: 1 },
    ]),
    cf('PROJ1', 'State', 'state', false, cbStates),
    cf('PROJ1', 'Deadline', 'date', false),
  ],
  // [] — the []-not-403 permissions trap
  '0-3': [],
};

let latest: any;

const Wrapper = ({ initial, scopeConfigId }: { initial: any; scopeConfigId?: string }) => {
  const [transformation, setTransformation] = useState<any>(initial);
  latest = transformation;
  return (
    <StyledThemeProvider theme={getTheme('light')}>
      <YoutrackTransformation
        entities={['TICKET', 'CROSS']}
        connectionId={1}
        scopeConfigId={scopeConfigId}
        transformation={transformation}
        setTransformation={setTransformation}
      />
    </StyledThemeProvider>
  );
};

const initialTransformation = {
  typeField: 'Client type',
  stateField: '',
  priorityField: '',
  assigneeField: '',
  storyPointField: '',
  dueDateField: '',
  typeMappings: { 'Fixed (old)': 'BUG' },
  statusMappings: { 'On hold': 'OTHER' },
};

describe('YoutrackTransformation (attached mode)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    scopeConfigProjects.mockResolvedValue({
      count: 1,
      projects: [
        {
          name: 'payments',
          blueprintId: 1,
          scopes: [
            { scopeId: '0-1', scopeName: 'Project C' },
            { scopeId: '0-2', scopeName: 'Project A' },
            { scopeId: '0-3', scopeName: 'Project B' },
          ],
        },
      ],
    });
    customFields.mockImplementation((_prefix: string, projectId: string) => Promise.resolve(FIELDS[projectId]));
  });

  it('unions bundle values by name — synonyms are separate rows with per-project presence badges', async () => {
    render(<Wrapper initial={initialTransformation} scopeConfigId="9" />);

    // synonyms survive as two rows (name-keyed mappings)
    expect(await screen.findByText('Subtask')).toBeDefined();
    expect(screen.getByText('Sub-task')).toBeDefined();

    // the type table follows the effective slot name
    expect(screen.getByText(/Union of the «Client type» bundle values/)).toBeDefined();

    // one proxy call per attached project, nothing more
    expect(customFields).toHaveBeenCalledTimes(3);
  });

  it('auto-seeds statuses from isResolved, never seeds types, and persists only via form state', async () => {
    render(<Wrapper initial={initialTransformation} scopeConfigId="9" />);

    await screen.findByText('Subtask');
    await waitFor(() => expect(latest.statusMappings['Closed']).toBe('DONE'));
    expect(latest.statusMappings['Backlog']).toBe('TODO');
    expect(latest.statusMappings['Declined']).toBe('DONE');
    expect(latest.statusMappings['State 2']).toBe('TODO');
    // a previously saved mapping is never overwritten by the seed
    expect(latest.statusMappings['On hold']).toBe('OTHER');
    // types are NOT seeded
    expect(latest.typeMappings).toEqual({ 'Fixed (old)': 'BUG' });

    // every unioned state carries the auto badge (34 rows: 2 + 33 states, Backlog shared by both projects)
    expect(screen.getAllByText('auto').length).toBe(34);
  });

  it('renders the permissions banner for a project whose customFields come back [], and keeps other projects', async () => {
    render(<Wrapper initial={initialTransformation} scopeConfigId="9" />);

    expect(await screen.findByText('Could not load custom fields for project Project B')).toBeDefined();
    expect(screen.getByText(/usually means the connection token lacks permission/)).toBeDefined();
    // other projects' rows still show
    expect(screen.getByText('Subtask')).toBeDefined();
  });

  it('badges mapped values gone upstream as stale and keeps them in the config', async () => {
    render(<Wrapper initial={initialTransformation} scopeConfigId="9" />);

    await screen.findByText('Subtask');
    // 'Fixed (old)' (type) and 'On hold' (status) are absent from every bundle
    expect(screen.getByText('Fixed (old)')).toBeDefined();
    expect(screen.getByText('On hold')).toBeDefined();
    // two stale rows × two badges each (presence column + mapping column)
    await waitFor(() => expect(screen.getAllByText('stale').length).toBe(4));
    // kept in the config
    expect(latest.typeMappings['Fixed (old)']).toBe('BUG');
    expect(latest.statusMappings['On hold']).toBe('OTHER');
  });

  it('scrolls with a sticky header when a table exceeds 30 rows', async () => {
    const { container } = render(<Wrapper initial={initialTransformation} scopeConfigId="9" />);

    await screen.findByText('State 32');
    // the 34-row status table gets the scroll region; the 5-row type table does not
    await waitFor(() => expect(container.querySelectorAll('.ant-table-body').length).toBe(1));
  });
});

describe('YoutrackTransformation (manual-entry mode)', () => {
  it('renders the hint and free-text entry when no scopeConfigId is given', async () => {
    render(<Wrapper initial={initialTransformation} />);

    expect(await screen.findByText(/manual entry mode/)).toBeDefined();
    expect(screen.getAllByPlaceholderText('Type the value name as it appears in YouTrack').length).toBe(2);
    // saved mappings render as plain rows without presence badges
    expect(screen.getByText('Fixed (old)')).toBeDefined();
    expect(screen.queryByText('PROJ2')).toBeNull();
    // an empty core slot shows the default-name placeholder; a set slot shows its value
    expect(screen.getByText('Default: State')).toBeDefined();
    expect(screen.getByDisplayValue('Client type')).toBeDefined();
    expect(screen.getAllByText('(none) — off').length).toBe(2);
  });
});
