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

import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';

import { AuthError } from '.';

const state = vi.hoisted(() => ({ methods: vi.fn() }));
vi.mock('@/api', () => ({ default: { auth: { methods: state.methods } } }));
vi.mock('@/config', () => ({ DEVLAKE_ENDPOINT: '/api' }));
vi.mock('@/components', () => ({ TipLayout: ({ children }: { children: React.ReactNode }) => <div>{children}</div> }));

function renderAt(search: string) {
  return render(
    <MemoryRouter initialEntries={[`/auth-error${search}`]}>
      <Routes>
        <Route path="/auth-error" element={<AuthError />} />
        <Route path="/login" element={<span>Login page</span>} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('auth error page', () => {
  beforeEach(() => {
    state.methods.mockReset().mockResolvedValue({});
  });

  it('explains a rejected account and offers access request and account switch', async () => {
    state.methods.mockResolvedValue({ accessRequestUrl: 'https://intranet.example.com/access' });
    const assign = vi.fn();
    const original = window.location;
    Object.defineProperty(window, 'location', {
      configurable: true,
      value: {
        ...original,
        set href(v: string) {
          assign(v);
        },
      },
    });

    try {
      renderAt('?code=forbidden&provider=keycloak');
      expect(screen.getByText('Access denied')).toBeTruthy();

      const request = await screen.findByRole('link', { name: 'Request access' });
      expect(request.getAttribute('href')).toBe('https://intranet.example.com/access');
      expect(request.getAttribute('rel')).toContain('noopener');
      expect(screen.queryByRole('button', { name: 'Back to sign in' })).toBeNull();

      fireEvent.click(screen.getByRole('button', { name: 'Sign in with a different account' }));
      expect(assign).toHaveBeenCalledWith('/api/auth/switch-account?provider=keycloak');
    } finally {
      Object.defineProperty(window, 'location', { configurable: true, value: original });
    }
  });

  it('hides the access request button when no URL is configured', async () => {
    renderAt('?code=forbidden');
    await waitFor(() => expect(state.methods).toHaveBeenCalled());
    expect(screen.queryByRole('link', { name: 'Request access' })).toBeNull();
    expect(screen.getByRole('button', { name: 'Sign in with a different account' })).toBeTruthy();
  });

  it.each([
    ['expired', 'Sign-in expired'],
    ['idp_error', 'Sign-in failed'],
    ['server_error', 'Something went wrong'],
    ['unknown-code', 'Something went wrong'],
    ['', 'Something went wrong'],
  ])('shows a message for code %j and links back to sign in', (code, title) => {
    renderAt(code ? `?code=${code}` : '');
    expect(screen.getByText(title)).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Sign in with a different account' })).toBeNull();
    expect(state.methods).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole('button', { name: 'Back to sign in' }));
    expect(screen.getByText('Login page')).toBeTruthy();
  });
});
