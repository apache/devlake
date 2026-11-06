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

import { ConnectionName } from './name';
import { ConnectionEndpoint } from './endpoint';
import { ConnectionUsername } from './username';
import { ConnectionPassword } from './password';
import { ConnectionToken } from './token';
import { ConnectionProxy } from './proxy';
import { ConnectionRateLimit } from './rate-limit';
import { ConnectionAppId } from './app-id';
import { ConnectionSecretKey } from './secret-key';
import { ConnectionTimezone } from './timezone';

interface Props {
  type: 'create' | 'update';
  name: string;
  fields: any[];
  initialValues: any;
  values: any;
  errors: any;
  setValues: (values: any) => void;
  setErrors: (errors: any) => void;
}

export const Form = ({ type, name, fields, initialValues, values, errors, setValues, setErrors }: Props) => {
  const onValues = (values: any) => setValues((prev: any) => ({ ...prev, ...values }));
  const onErrors = (values: any) => setErrors((prev: any) => ({ ...prev, ...values }));

  const getProps = (key: string, defaultValue: any = '') => {
    return {
      name,
      initialValue: initialValues?.[key] ?? defaultValue,
      value: values?.[key] ?? defaultValue,
      error: errors?.[key] ?? defaultValue,
      setValue: (value: any) => onValues({ [key]: value }),
      setError: (value: any) => onErrors({ [key]: value }),
    };
  };

  const generateForm = () => {
    return fields.map((field) => {
      if (typeof field === 'function') {
        return field({
          type,
          initialValues,
          values,
          setValues: onValues,
          errors,
          setErrors: onErrors,
        });
      }

      const key = typeof field === 'string' ? field : field.key;

      switch (key) {
        case 'name':
          return <ConnectionName key={key} {...getProps('name')} {...field} />;
        case 'endpoint':
          return <ConnectionEndpoint key={key} {...getProps('endpoint')} {...field} />;
        case 'username':
          return <ConnectionUsername key={key} {...getProps('username')} {...field} />;
        case 'password':
          return <ConnectionPassword key={key} type={type} {...getProps('password')} {...field} />;
        case 'token':
          return <ConnectionToken key={key} type={type} {...getProps('token')} {...field} />;
        case 'appId':
          return <ConnectionAppId key={key} {...getProps('appId')} {...field} />;
        case 'secretKey':
          return <ConnectionSecretKey key={key} type={type} {...getProps('secretKey')} {...field} />;
        case 'proxy':
          return <ConnectionProxy key={key} {...getProps('proxy')} {...field} />;
        case 'rateLimitPerHour':
          return <ConnectionRateLimit key={key} {...getProps('rateLimitPerHour', 0)} {...field} />;
        case 'timezone':
          return <ConnectionTimezone key={key} {...getProps('timezone')} {...field} />;
        default:
          return null;
      }
    });
  };

  return <div>{generateForm()}</div>;
};
