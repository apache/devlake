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

import { useEffect, useState } from 'react';
import { Select } from 'antd';

import API from '@/api';
import { Block } from '@/components';

interface Props {
  initialValue: number;
  value: number;
  error: string;
  setValue: (value: number) => void;
  setError: (error: string) => void;
}

export const GitHubConnectionSelect = ({ initialValue, value, error, setValue, setError }: Props) => {
  const [connections, setConnections] = useState<Array<{ id: number; name: string }>>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    setLoading(true);
    API.connection
      .list('github')
      .then((conns) => setConnections(conns.map((c: any) => ({ id: c.id, name: c.name }))))
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => {
    if (initialValue && !value) {
      setValue(initialValue);
    }
  }, [initialValue]);

  useEffect(() => {
    if (value === 0 || value === undefined) {
      setError('A GitHub connection is required');
    } else {
      setError('');
    }
  }, [value]);

  return (
    <Block
      title="GitHub Connection"
      description="Select an existing GitHub connection to authenticate API calls to the submissions repository."
      required
    >
      <Select
        style={{ width: 386 }}
        loading={loading}
        placeholder="Select a GitHub connection"
        value={value || undefined}
        onChange={(val) => setValue(val)}
        options={connections.map((c) => ({ label: c.name, value: c.id }))}
      />
    </Block>
  );
};
