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

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';

const { remote, searchRemote } = vi.hoisted(() => ({
  remote: vi.fn(),
  searchRemote: vi.fn(),
}));

vi.mock('@/api', () => ({ default: { scope: { remote, searchRemote } } }));
vi.mock('@/plugins', () => ({ getPluginScopeName: () => undefined }));
vi.mock('@/components', () => ({
  Loading: () => <span>loading</span>,
  Block: ({ title, children }: any) => (
    <div>
      {title}
      {children}
    </div>
  ),
}));

import { SearchRemote } from './search-remote';

const project = (i: number) => ({ id: `p${i}`, name: `proj-${i}`, type: 'scope', parentId: null, data: { key: i } });

const renderComponent = (props: Partial<Parameters<typeof SearchRemote>[0]> = {}) => {
  const onChange = vi.fn();
  render(
    <SearchRemote
      mode="multiple"
      plugin="sonarqube"
      connectionId={1}
      config={{ title: 'Repositories', searchSelectAll: true }}
      disabledScope={[]}
      selectedScope={[]}
      onChange={onChange}
      {...props}
    />,
  );
  return onChange;
};

const typeSearch = (value: string) => fireEvent.change(screen.getByPlaceholderText('Search'), { target: { value } });

const findAllCheckbox = async () =>
  (await screen.findByRole('checkbox', { name: /^ALL/ }, { timeout: 2000 })) as HTMLInputElement;

describe('SearchRemote "ALL" toggle', () => {
  beforeEach(() => {
    remote.mockReset().mockResolvedValue({ children: [], nextPageToken: '' });
    searchRemote.mockReset();
  });

  it('is hidden without a search query', async () => {
    renderComponent();
    await waitFor(() => expect(remote).toHaveBeenCalled());
    expect(screen.queryByText(/^ALL/)).toBeNull();
  });

  it('is hidden when the plugin does not enable it', async () => {
    searchRemote.mockResolvedValue({ children: [project(1)] });
    renderComponent({ config: { title: 'Repositories' } });
    typeSearch('proj');
    await screen.findByText('proj-1', undefined, { timeout: 2000 });
    expect(screen.queryByText(/^ALL/)).toBeNull();
  });

  it('is rendered as the first entry inside the result list', async () => {
    searchRemote.mockResolvedValueOnce({ children: [project(1), project(2)], count: 2 });
    renderComponent();
    typeSearch('proj');

    const checkbox = await findAllCheckbox();
    const row = screen.getByTestId('search-select-all');
    const column = document.getElementById('miller-columns-column-root');
    expect(column).not.toBeNull();
    expect(column!.contains(row)).toBe(true);

    const firstItem = screen.getByText('proj-1');
    // the "ALL" row precedes the first item in document order
    expect(row.compareDocumentPosition(firstItem) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(checkbox.closest('label')?.textContent).toBe('ALL (2)');
  });

  it('selects every filtered item, loading all remaining pages', async () => {
    // first page from the regular (debounced) search: a full page => more results exist
    const firstPage = Array.from({ length: 50 }, (_, i) => project(i));
    searchRemote.mockResolvedValueOnce({ children: firstPage });
    // "ALL" click: loads everything with its own page size
    searchRemote.mockResolvedValueOnce({ children: Array.from({ length: 100 }, (_, i) => project(i)) });
    searchRemote.mockResolvedValueOnce({ children: [project(100), project(101)] });

    const onChange = renderComponent({ selectedScope: [{ id: 'existing' }] });
    typeSearch('proj');

    const checkbox = await findAllCheckbox();
    await waitFor(() => expect(checkbox.disabled).toBe(false));
    fireEvent.click(checkbox);

    await waitFor(() => expect(onChange).toHaveBeenCalled());
    expect(searchRemote).toHaveBeenCalledTimes(3);
    expect(searchRemote).toHaveBeenLastCalledWith('sonarqube', 1, { search: 'proj', page: 2, pageSize: 100 });

    const selected = onChange.mock.calls[0][0];
    expect(selected).toHaveLength(103);
    expect(selected[0].id).toBe('existing');
    expect(selected.map((it: any) => it.id)).toContain('p101');
  });

  it('stops loading once the backend-reported count is reached', async () => {
    searchRemote.mockResolvedValueOnce({ children: Array.from({ length: 50 }, (_, i) => project(i)), count: 200 });
    searchRemote.mockResolvedValueOnce({ children: Array.from({ length: 100 }, (_, i) => project(i)), count: 200 });
    searchRemote.mockResolvedValueOnce({
      children: Array.from({ length: 100 }, (_, i) => project(i + 100)),
      count: 200,
    });

    const onChange = renderComponent();
    typeSearch('proj');

    const checkbox = await findAllCheckbox();
    expect(checkbox.closest('label')?.textContent).toBe('ALL (200)');
    await waitFor(() => expect(checkbox.disabled).toBe(false));
    fireEvent.click(checkbox);

    await waitFor(() => expect(onChange).toHaveBeenCalled());
    // 1 regular search + 2 pages for "ALL" (no extra request for an empty third page)
    expect(searchRemote).toHaveBeenCalledTimes(3);
    expect(onChange.mock.calls[0][0]).toHaveLength(200);
  });

  it('deselects the filtered items when all are already selected and skips disabled ones', async () => {
    searchRemote.mockResolvedValueOnce({ children: [project(1), project(2), project(3)] });

    const onChange = renderComponent({
      selectedScope: [{ id: 'existing' }, project(1), project(3)],
      disabledScope: [{ id: 'p2' }],
    });
    typeSearch('proj');

    const checkbox = await findAllCheckbox();
    await waitFor(() => expect(checkbox.checked).toBe(true));
    fireEvent.click(checkbox);

    // the complete list was already known => no additional request
    expect(searchRemote).toHaveBeenCalledTimes(1);
    await waitFor(() => expect(onChange).toHaveBeenCalledWith([{ id: 'existing' }]));
  });
});
