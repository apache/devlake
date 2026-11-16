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

// ---------------------------------------------------------------------------
// Regex constants (mirror exactly what auth.tsx defines)
// ---------------------------------------------------------------------------
const JIRA_CLOUD_REGEX = /^https:\/\/\w+\.atlassian\.net\/rest\/$/;
const JIRA_GATEWAY_REGEX = /^https:\/\/api\.atlassian\.com\/ex\/jira\/[^/]+\/rest\/$/;

// ---------------------------------------------------------------------------
// Endpoint builder (mirrors handleChangeCloudId in auth.tsx)
// ---------------------------------------------------------------------------
function buildGatewayEndpoint(cloudId: string): string {
  return cloudId ? `https://api.atlassian.com/ex/jira/${cloudId}/rest/` : '';
}

// ---------------------------------------------------------------------------
// Cloud ID extractor (mirrors the load-time useEffect in auth.tsx)
// ---------------------------------------------------------------------------
function extractCloudId(endpoint: string): string | null {
  const match = endpoint.match(/\/ex\/jira\/([^/]+)\//);
  return match ? match[1] : null;
}

// ---------------------------------------------------------------------------
// JIRA_GATEWAY_REGEX tests
// ---------------------------------------------------------------------------
describe('JIRA_GATEWAY_REGEX', () => {
  it('accepts a valid gateway URL with UUID Cloud ID', () => {
    expect(
      JIRA_GATEWAY_REGEX.test('https://api.atlassian.com/ex/jira/a1b2c3d4-e5f6-7890-abcd-ef1234567890/rest/'),
    ).toBe(true);
  });

  it('accepts a valid gateway URL with short Cloud ID', () => {
    expect(JIRA_GATEWAY_REGEX.test('https://api.atlassian.com/ex/jira/mycloud123/rest/')).toBe(true);
  });

  it('rejects gateway URL missing trailing slash', () => {
    expect(JIRA_GATEWAY_REGEX.test('https://api.atlassian.com/ex/jira/abc123/rest')).toBe(false);
  });

  it('rejects gateway URL missing /rest/', () => {
    expect(JIRA_GATEWAY_REGEX.test('https://api.atlassian.com/ex/jira/abc123/')).toBe(false);
  });

  it('rejects standard Jira Cloud URL', () => {
    expect(JIRA_GATEWAY_REGEX.test('https://mycompany.atlassian.net/rest/')).toBe(false);
  });

  it('rejects Jira Server URL', () => {
    expect(JIRA_GATEWAY_REGEX.test('https://jira.mycompany.com/rest/')).toBe(false);
  });

  it('rejects gateway URL with extra path segment after /rest/', () => {
    expect(JIRA_GATEWAY_REGEX.test('https://api.atlassian.com/ex/jira/abc123/rest/extra/')).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// JIRA_CLOUD_REGEX regression tests — must not break existing behaviour
// ---------------------------------------------------------------------------
describe('JIRA_CLOUD_REGEX (regression)', () => {
  it('accepts a standard Jira Cloud URL', () => {
    expect(JIRA_CLOUD_REGEX.test('https://mycompany.atlassian.net/rest/')).toBe(true);
  });

  it('rejects gateway URL', () => {
    expect(JIRA_CLOUD_REGEX.test('https://api.atlassian.com/ex/jira/abc123/rest/')).toBe(false);
  });

  it('rejects Jira Server URL', () => {
    expect(JIRA_CLOUD_REGEX.test('https://jira.mycompany.com/rest/')).toBe(false);
  });

  it('rejects cloud URL missing trailing slash', () => {
    expect(JIRA_CLOUD_REGEX.test('https://mycompany.atlassian.net/rest')).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// buildGatewayEndpoint tests
// ---------------------------------------------------------------------------
describe('buildGatewayEndpoint', () => {
  it('builds the correct URL from a UUID Cloud ID', () => {
    expect(buildGatewayEndpoint('a1b2c3d4-e5f6-7890-abcd-ef1234567890')).toBe(
      'https://api.atlassian.com/ex/jira/a1b2c3d4-e5f6-7890-abcd-ef1234567890/rest/',
    );
  });

  it('builds the correct URL from a short Cloud ID', () => {
    expect(buildGatewayEndpoint('mycloud123')).toBe('https://api.atlassian.com/ex/jira/mycloud123/rest/');
  });

  it('returns empty string for empty Cloud ID', () => {
    expect(buildGatewayEndpoint('')).toBe('');
  });

  it('produced URL matches JIRA_GATEWAY_REGEX', () => {
    const url = buildGatewayEndpoint('test-cloud-id');
    expect(JIRA_GATEWAY_REGEX.test(url)).toBe(true);
  });
});

// ---------------------------------------------------------------------------
// extractCloudId tests — verifies the edit-mode useEffect can pre-fill Cloud ID
// ---------------------------------------------------------------------------
describe('extractCloudId', () => {
  it('extracts UUID Cloud ID from a saved gateway endpoint', () => {
    expect(extractCloudId('https://api.atlassian.com/ex/jira/a1b2c3d4-e5f6-7890-abcd-ef1234567890/rest/')).toBe(
      'a1b2c3d4-e5f6-7890-abcd-ef1234567890',
    );
  });

  it('extracts short Cloud ID', () => {
    expect(extractCloudId('https://api.atlassian.com/ex/jira/mycloud123/rest/')).toBe('mycloud123');
  });

  it('returns null for a standard cloud URL', () => {
    expect(extractCloudId('https://mycompany.atlassian.net/rest/')).toBeNull();
  });

  it('round-trips: build then extract returns original Cloud ID', () => {
    const id = 'round-trip-id-123';
    expect(extractCloudId(buildGatewayEndpoint(id))).toBe(id);
  });
});
