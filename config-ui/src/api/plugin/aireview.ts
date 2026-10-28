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

import { request } from '@/utils';

export interface IAiReviewScopeConfig {
  id?: number;
  name?: string;

  // CodeRabbit detection
  codeRabbitEnabled: boolean;
  codeRabbitUsername: string;
  codeRabbitPattern: string;

  // Cursor Bugbot detection
  cursorBugbotEnabled: boolean;
  cursorBugbotUsername: string;
  cursorBugbotPattern: string;

  // Qodo detection
  qodoEnabled: boolean;
  qodoUsername: string;
  qodoPattern: string;

  // Gemini Code Assist detection
  geminiEnabled: boolean;
  geminiUsername: string;
  geminiPattern: string;

  // Generic AI detection
  aiCommitPatterns: string;
  aiPrLabelPattern: string;

  // Risk detection
  riskHighPattern: string;
  riskMediumPattern: string;
  riskLowPattern: string;

  // Thresholds
  observationWindowDays: number;
  warningThreshold: number;

  // CI failure source: 'test_cases' | 'job_result' | 'both'
  ciFailureSource: string;

  // CI failure filtering
  excludeFlakyTests: boolean;
  excludeInfraFailures: boolean;

  // Issue linking
  bugLinkPattern: string;
}

export const getDefaultScopeConfig = (): Promise<IAiReviewScopeConfig> =>
  request('/plugins/aireview/scope-configs/default');

export const getScopeConfig = (id: number): Promise<IAiReviewScopeConfig> =>
  request(`/plugins/aireview/scope-configs/${id}`);

export const createScopeConfig = (data: Partial<IAiReviewScopeConfig>): Promise<IAiReviewScopeConfig> =>
  request('/plugins/aireview/scope-configs', { method: 'post', data });

export const updateScopeConfig = (id: number, data: Partial<IAiReviewScopeConfig>): Promise<IAiReviewScopeConfig> =>
  request(`/plugins/aireview/scope-configs/${id}`, { method: 'patch', data });
