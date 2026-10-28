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

export const SubmissionsPath = ({ initialValue, value, setValue }: Props) => {
  useEffect(() => {
    if (initialValue && !value) {
      setValue(initialValue);
    }
  }, [initialValue]);

  return (
    <Block
      title="Submissions Path"
      description="Subdirectory within the repo containing {org}/{repo}/{file}.json structure."
    >
      <Input
        style={{ width: 386 }}
        placeholder="submissions"
        value={value}
        onChange={(e) => setValue(e.target.value)}
      />
    </Block>
  );
};
