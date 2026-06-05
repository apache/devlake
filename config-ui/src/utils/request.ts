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

import type { AxiosRequestConfig } from 'axios';
import axios from 'axios';

import { DEVLAKE_ENDPOINT } from '@/config';

const instance = axios.create({
  baseURL: DEVLAKE_ENDPOINT,
  withCredentials: true,
  // Double-submit CSRF: axios reads `devlake_csrf` and echoes it as
  // `X-CSRF-Token` on unsafe methods. The cookie is set by /auth/callback.
  xsrfCookieName: 'devlake_csrf',
  xsrfHeaderName: 'X-CSRF-Token',
});

export type RequestConfig = {
  baseURL?: string;
  method?: AxiosRequestConfig['method'];
  data?: unknown;
  timeout?: number;
  signal?: AbortSignal;
  headers?: Record<string, string>;
};

const isLoginRoute = () => window.location.pathname.replace(/\/+$/, '').endsWith('/login');

let redirectingToLogin = false;

instance.interceptors.response.use(
  (response) => response,
  (error) => {
    const status = error.response?.status;

    if (status === 428) {
      window.location.replace('/db-migrate');
    }

    if (status === 401 && !isLoginRoute() && !redirectingToLogin) {
      redirectingToLogin = true;
      const returnUrl = encodeURIComponent(window.location.pathname + window.location.search);
      window.location.replace(`/login?return_url=${returnUrl}`);
    }

    return Promise.reject(error);
  },
);

export const request = (path: string, config?: RequestConfig) => {
  const { method = 'get', data, timeout, headers, signal } = config || {};

  const cancelTokenSource = axios.CancelToken.source();
  const token = localStorage.getItem('accessToken');
  const h = { ...headers };
  if (token) {
    h.Authorization = `Bearer ${token}`;
  }

  const params: any = {
    baseURL: config?.baseURL,
    url: path,
    method,
    timeout,
    headers: h,
    cancelToken: cancelTokenSource?.token,
  };

  if (['GET', 'get'].includes(method)) {
    params.params = data;
  } else {
    params.data = data;
  }

  const promise = instance.request(params).then((resp) => resp.data);

  if (signal) {
    signal.addEventListener('abort', () => {
      cancelTokenSource?.cancel();
    });
  }

  return promise;
};
