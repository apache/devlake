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

interface Props {
  initialValue: string;
  value: string;
  error: string;
  setValue: (value: string) => void;
  setError: (error: string) => void;
}

export const GitHubToken = ({ initialValue, value, error, setValue, setError }: Props) => {
  // Initialize with initialValue when editing
  // Note: Token might be sanitized (showing ***), so we check if it's different
  useEffect(() => {
    if (initialValue && (!value || value === initialValue)) {
      // Only set if value is empty or same as initial (not user-modified)
      setValue(initialValue);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [initialValue]);

  // Validate on value change (only if value exists or was changed by user)
  useEffect(() => {
    // Skip validation on initial mount if value is empty (allows form to start without errors)
    if (value === '' && !initialValue) {
      return;
    }
    
    if (!value || value.trim() === '') {
      setError('GitHub token is required');
    } else {
      setError('');
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value]);

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setValue(e.target.value);
  };

  return (
    <Block
      title="GitHub Token"
      description="Enter your GitHub personal access token to access GitHub repositories and fetch data scopes. The token will be encrypted in the database."
      required
      error={error}
    >
      <Input.Password
        style={{ width: 386 }}
        placeholder="ghp_xxxxxxxxxxxxxxxxxxxx"
        value={value}
        onChange={handleChange}
      />
    </Block>
  );
};

