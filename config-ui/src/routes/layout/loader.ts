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

import { redirect } from 'react-router-dom';
import { intersection } from 'lodash';

import API from '@/api';
import { getRegisterPlugins } from '@/plugins';

type Props = {
  request: Request;
};

export const layoutLoader = async ({ request }: Props) => {
  const onboard = await API.store.get('onboard');

  if (!onboard) {
    return redirect('/onboard');
  }

  let fePlugins = getRegisterPlugins();
  try {
    const envPlugins = import.meta.env.DEVLAKE_PLUGINS.split(',').filter(Boolean);
    fePlugins = fePlugins.filter((plugin) => !envPlugins.length || envPlugins.includes(plugin));
  } catch (err) {}

  const [bePlugins, res, user] = await Promise.all([
    API.plugin.list(),
    API.version(request.signal),
    API.auth.userinfo().catch(() => null),
  ]);

  return {
    version: res.version,
    plugins: intersection(
      fePlugins,
      bePlugins.map((it) => it.plugin),
    ),
    user,
  };
};
