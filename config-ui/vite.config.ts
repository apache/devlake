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

// Allow DevLake API access from the dev server when using Dev Container or custom host/port
const devlakeOrigin = process.env.VITE_DEVLAKE_URL || 'http://localhost:8080';

// Allow Grafana access from the dev server when using Dev Container
const grafanaProtocol = process.env.VITE_GRAFANA_PROTOCOL || 'http';
const grafanaHost = process.env.VITE_GRAFANA_HOST || 'localhost';
const grafanaPort = process.env.VITE_GRAFANA_PORT || '3001';
const grafanaOrigin = process.env.VITE_GRAFANA_URL || `${grafanaProtocol}://${grafanaHost}:${grafanaPort}`;
const grafanaChangeOrigin = envBool('VITE_GRAFANA_CHANGE_ORIGIN', true);

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react(), svgr()],

  envPrefix: 'DEVLAKE_',

  server: {
    port: 4000,
    proxy: {
      '/api': {
        target: devlakeOrigin,
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api\//, ''),
      },
      '/grafana': {
        target: grafanaOrigin,
        changeOrigin: grafanaChangeOrigin,
        ws: true, // Proxying websockets to allow features like query auto-complete
        configure: (proxy) => {
          // Grafana's CSRF protection rejects state-changing requests (e.g.
          // starring a dashboard, running panel queries) whose Origin/Referer
          // host doesn't match the request Host, returning "origin not allowed".
          // Because changeOrigin rewrites Host to the Grafana target, rewrite
          // Origin/Referer to match so the same-origin check passes.
          proxy.on('proxyReq', (proxyReq) => {
            if (proxyReq.getHeader('origin')) {
              proxyReq.setHeader('origin', grafanaOrigin);
            }
            const referer = proxyReq.getHeader('referer');
            if (typeof referer === 'string') {
              proxyReq.setHeader('referer', referer.replace(/^https?:\/\/[^/]+/, grafanaOrigin));
            }
          });
        },
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
