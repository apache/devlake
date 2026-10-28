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

export const SubmissionsRepo = ({ initialValue, value, error, setValue, setError }: Props) => {
  useEffect(() => {
    if (initialValue && !value) {
      setValue(initialValue);
    }
  }, [initialValue]);

  useEffect(() => {
    if (!value || value.trim() === '') {
      setError('Submissions repository is required');
    } else if (!value.includes('/')) {
      setError('Must be in owner/repo format');
    } else {
      setError('');
    }
  }, [value]);

  return (
    <Block
      title="Submissions Repository"
      description="GitHub repository containing assessment submissions (e.g., ambient-code/agentready)."
      required
    >
      <Input style={{ width: 386 }} placeholder="org/repo" value={value} onChange={(e) => setValue(e.target.value)} />
    </Block>
  );
};
