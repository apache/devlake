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

import ReactDOM from 'react-dom';
import { Provider } from 'react-redux';
import { RouterProvider } from 'react-router-dom';

import { PageLoading } from '@/components';
import { ThemeProvider } from '@/theme';

import { store } from './app/store';
import { router } from './app/router';
import './index.css';

ReactDOM.render(
  <Provider store={store}>
    <ThemeProvider>
      <RouterProvider router={router} fallbackElement={<PageLoading />} />
    </ThemeProvider>
  </Provider>,
  document.getElementById('root'),
);
