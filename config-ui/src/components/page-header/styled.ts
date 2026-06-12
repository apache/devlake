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

import styled from 'styled-components';

export const Container = styled.div``;

export const Title = styled.div`
  display: flex;
  align-items: center;
  justify-content: space-between;
`;

export const Description = styled.p`
  margin-top: 12px;
  margin-bottom: 16px;
`;

export const Content = styled.div``;

export const Breadcrumbs = styled.ul`
  display: flex;
  align-items: center;
`;

export const Breadcrumb = styled.li`
  display: flex;
  align-items: center;
  font-size: 24px;
  font-weight: 600;

  a {
    display: flex;
    align-items: center;
    color: ${({ theme }) => theme.colors.text};
  }
`;
export const Extra = styled.div``;
