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

import { Select } from 'antd';
import { useEffect } from 'react';

import { Block } from '@/components';

interface Props {
  type: 'create' | 'update';
  initialValues: any;
  values: any;
  errors: any;
  setValues: (value: any) => void;
  setErrors: (value: any) => void;
}

const serviceOptions = [
  { value: 'github', label: 'GitHub' },
  { value: 'gitlab', label: 'GitLab' },
  { value: 'github_enterprise', label: 'GitHub Enterprise' },
  { value: 'gitlab_enterprise', label: 'GitLab Enterprise' },
  { value: 'bitbucket', label: 'Bitbucket' },
  { value: 'bitbucket_server', label: 'Bitbucket Server' },
];

export const Service = ({ initialValues, values, setValues }: Props) => {
  useEffect(() => {
    setValues({ service: initialValues.service || 'github' });
  }, [initialValues.service]);

  const handleChange = (value: string) => {
    setValues({ service: value });
  };

  return (
    <Block
      title="Service"
      description="Select the type of Git hosting platform integrated with your Codecov instance. Use 'GitHub' for github.com repos (even if Codecov itself is self-hosted), 'GitHub Enterprise' only for self-hosted GitHub Enterprise Server. Same logic applies to GitLab/Bitbucket variants."
      required
    >
      <Select
        style={{ width: 386 }}
        value={values.service || 'github'}
        onChange={handleChange}
        options={serviceOptions}
      />
    </Block>
  );
};
