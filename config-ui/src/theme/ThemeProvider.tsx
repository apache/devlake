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

import { ReactNode, useEffect, useMemo, useState } from 'react';
import { ConfigProvider } from 'antd';
import { ThemeProvider as StyledThemeProvider } from 'styled-components';

import { useAppSelector } from '@/hooks';
import { selectThemeMode, resolveThemeMode } from '@/features/theme/slice';

import { getTheme, ResolvedTheme } from './tokens';

interface Props {
  children: ReactNode;
}

const CSS_VAR_MAP: Record<string, keyof ReturnType<typeof getTheme>['colors']> = {
  '--devlake-color-text': 'text',
  '--devlake-color-text-muted': 'textTertiary',
  '--devlake-color-text-body': 'textBody',
  '--devlake-color-text-subdued': 'textMuted',
  '--devlake-color-bg': 'bgContainer',
  '--devlake-color-bg-elevated': 'bgElevated',
  '--devlake-color-border': 'border',
  '--devlake-color-success': 'success',
  '--devlake-color-error': 'error',
  '--devlake-color-error-alt': 'errorAlt',
  '--devlake-color-warning': 'warning',
  '--devlake-color-warning-alt': 'warningAlt',
  '--devlake-color-warning-soft': 'warningSoft',
};

export const ThemeProvider = ({ children }: Props) => {
  const mode = useAppSelector(selectThemeMode);
  const [resolved, setResolved] = useState<ResolvedTheme>(() => resolveThemeMode(mode));

  useEffect(() => {
    setResolved(resolveThemeMode(mode));

    if (mode !== 'system' || typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
      return;
    }

    const media = window.matchMedia('(prefers-color-scheme: dark)');
    const onChange = () => setResolved(media.matches ? 'dark' : 'light');
    media.addEventListener('change', onChange);
    return () => media.removeEventListener('change', onChange);
  }, [mode]);

  const customPrimary = import.meta.env.DEVLAKE_COLOR_CUSTOM;
  const theme = useMemo(() => getTheme(resolved, customPrimary), [resolved, customPrimary]);

  useEffect(() => {
    if (typeof document === 'undefined') return;
    const root = document.documentElement;
    root.dataset.theme = resolved;
    root.style.colorScheme = resolved;
    for (const [cssVar, key] of Object.entries(CSS_VAR_MAP)) {
      root.style.setProperty(cssVar, theme.colors[key]);
    }
  }, [resolved, theme]);

  return (
    <ConfigProvider theme={theme.antd}>
      <StyledThemeProvider theme={theme}>{children}</StyledThemeProvider>
    </ConfigProvider>
  );
};
