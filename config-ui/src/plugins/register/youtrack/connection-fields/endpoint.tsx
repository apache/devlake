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

import { useEffect } from 'react';
import { Input } from 'antd';

import { Block } from '@/components';

const ENDPOINT_EXAMPLE = 'https://example.myjetbrains.com/youtrack/api';

const INVALID_MESSAGE = `endpoint must be the full API base URL ending in /api — e.g. ${ENDPOINT_EXAMPLE}`;

// validateEndpoint mirrors the backend rule: after trimming trailing slashes,
// the URL must be absolute http(s) with a path ending in `/api`. The endpoint
// is never inferred from the hostname — self-hosted shapes are unconstrained.
export const validateEndpoint = (endpoint: string): string => {
  if (!endpoint) {
    return 'endpoint is required';
  }
  const normalized = endpoint.replace(/\/+$/, '');
  let url: URL;
  try {
    url = new URL(normalized);
  } catch {
    return INVALID_MESSAGE;
  }
  if (url.protocol !== 'http:' && url.protocol !== 'https:') {
    return INVALID_MESSAGE;
  }
  if (!url.pathname.endsWith('/api')) {
    return INVALID_MESSAGE;
  }
  return '';
};

interface Props {
  type: 'create' | 'update';
  initialValues: any;
  values: any;
  errors: any;
  setValues: (value: any) => void;
  setErrors: (value: any) => void;
}

export const Endpoint = ({ initialValues, values, errors, setValues, setErrors }: Props) => {
  useEffect(() => {
    setValues({ endpoint: initialValues.endpoint ?? '' });
  }, [initialValues.endpoint]);

  useEffect(() => {
    setErrors({ endpoint: validateEndpoint(values.endpoint ?? '') });
  }, [values.endpoint]);

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setValues({ endpoint: e.target.value });
  };

  // like the built-in fields, an empty input only disables the buttons; a
  // malformed one also says why, with the canonical example
  const showError = !!values.endpoint && !!errors?.endpoint;

  return (
    <Block
      title="Endpoint URL"
      description={
        <>
          The full API base URL of your YouTrack instance — it must end in <code>/api</code>:
          <ul style={{ margin: '4px 0 0', paddingLeft: 20 }}>
            <li>
              InCloud: <code>https://example.myjetbrains.com/youtrack/api</code> — note the <code>/youtrack</code>{' '}
              context path, the most common setup error
            </li>
            <li>
              YouTrack Cloud: <code>https://example.youtrack.cloud/api</code>
            </li>
            <li>
              Self-hosted: <code>https://youtrack.example.com/api</code>, or your custom context path such as{' '}
              <code>https://www.example.com/youtrack/api</code>
            </li>
          </ul>
        </>
      }
      required
    >
      <Input
        style={{ width: 480 }}
        status={showError ? 'error' : ''}
        placeholder={ENDPOINT_EXAMPLE}
        value={values.endpoint}
        onChange={handleChange}
      />
      {showError && <p style={{ margin: '4px 0 0', color: '#ff4d4f' }}>{errors.endpoint}</p>}
    </Block>
  );
};
