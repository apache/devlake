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

import path from 'path';
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import svgr from 'vite-plugin-svgr';

// Allow Grafana access from the dev server when using Dev Container
const grafanaOrigin = process.env.VITE_GRAFANA_URL || 'http://localhost:3002';
const grafanaChangeOrigin = envBool('VITE_GRAFANA_CHANGE_ORIGIN', true);

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react(), svgr()],

  envPrefix: 'DEVLAKE_',

  server: {
    port: 4000,
    proxy: {
      '/api': {
        target: 'http://localhost:8080/',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api\//, ''),
      },
      '/grafana': {
        target: grafanaOrigin,
        changeOrigin: grafanaChangeOrigin,
        ws: true, // Proxying websockets to allow features like query auto-complete
      },
    },
  },

  resolve: {
    alias: {
      '@': path.join(__dirname, './src'),
    },
  },
});

function envBool(name: string, defaultValue = false): boolean {
  const v = process.env[name];
  if (v == null) return defaultValue;
  return /^(1|true|yes|on)$/i.test(v);
}
