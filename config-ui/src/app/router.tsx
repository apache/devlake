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

import { createBrowserRouter, Navigate } from 'react-router-dom';

import {
  DBMigrate,
  Onboard,
  Error,
  Layout,
  layoutLoader,
  Login,
  Connections,
  Connection,
  ProjectHomePage,
  ProjectDetailPage,
  BlueprintHomePage,
  BlueprintDetailPage,
  BlueprintConnectionDetailPage,
  Pipelines,
  Pipeline,
  ApiKeys,
  NotFound,
} from '@/routes';

const PATH_PREFIX = import.meta.env.DEVLAKE_PATH_PREFIX ?? '';

export const router = createBrowserRouter([
  {
    path: '/',
    element: <Navigate to={PATH_PREFIX ? PATH_PREFIX : '/connections'} />,
  },
  {
    path: `${PATH_PREFIX}/db-migrate`,
    element: <DBMigrate />,
  },
  {
    path: `${PATH_PREFIX}/login`,
    element: <Login />,
  },
  {
    path: `${PATH_PREFIX}/onboard`,
    element: <Onboard />,
  },
  {
    path: `${PATH_PREFIX}`,
    element: <Layout />,
    loader: layoutLoader,
    errorElement: <Error />,
    children: [
      {
        index: true,
        element: <Navigate to="projects" />,
      },
      {
        path: 'projects',
        element: <ProjectHomePage />,
      },
      {
        path: 'projects/:pname',
        element: <ProjectDetailPage />,
      },
      {
        path: 'projects/:pname/:unique',
        element: <BlueprintConnectionDetailPage />,
      },
      {
        path: 'connections',
        element: <Connections />,
      },
      {
        path: 'connections/:plugin/:id',
        element: <Connection />,
      },
      {
        path: 'advanced',
        children: [
          {
            path: 'blueprints',
            element: <BlueprintHomePage />,
          },
          {
            path: 'blueprints/:id',
            element: <BlueprintDetailPage />,
          },
          {
            path: 'blueprints/:bid/:unique',
            element: <BlueprintConnectionDetailPage />,
          },
          {
            path: 'pipelines',
            element: <Pipelines />,
          },
          {
            path: 'pipeline/:id',
            element: <Pipeline />,
          },
        ],
      },
      {
        path: 'keys',
        element: <ApiKeys />,
      },
    ],
  },
  {
    path: '*',
    element: <NotFound />,
  },
]);
