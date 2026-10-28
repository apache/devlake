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

import { Input } from 'antd';

import { Block, ExternalLink } from '@/components';
import { useEffect } from 'react';

interface Props {
  type: 'create' | 'update';
  initialValues: any;
  values: any;
  errors: any;
  setValues: (value: any) => void;
  setErrors: (value: any) => void;
}

export const Organization = ({ type, initialValues, values, setValues, setErrors }: Props) => {
  useEffect(() => {
    setValues({ organization: initialValues.organization });
  }, [initialValues.organization]);

  useEffect(() => {
    setErrors({
      organization: values.organization ? '' : 'organization is required',
    });
  }, [values.organization]);

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setValues({
      organization: e.target.value,
    });
  };

  return (
    <Block
      title="Organization Owner"
      description={
        <>
          Enter the Codecov organization owner name. This is the organization, group, or username on your code hosting
          service (GitHub org, GitLab group, etc.) as registered in Codecov.{' '}
          <ExternalLink link="https://codecov.io">Learn more about Codecov</ExternalLink>
        </>
      }
      required
    >
      <Input style={{ width: 386 }} placeholder="e.g. my-org" value={values.organization} onChange={handleChange} />
    </Block>
  );
};

