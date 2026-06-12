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

export const StatusWrapper = styled.div`
  &.ready,
  &.cancel {
    color: ${({ theme }) => theme.colors.textTertiary};
  }

  &.loading {
    color: ${({ theme }) => theme.colors.primary};
  }

  &.success {
    color: ${({ theme }) => theme.colors.success};
  }

  &.error {
    color: ${({ theme }) => theme.colors.error};
  }
`;

export const Info = styled.div`
  ul {
    display: flex;
    align-items: center;
  }

  li {
    flex: 5;
    display: flex;
    flex-direction: column;

    &:last-child {
      flex: 1;
    }

    & > span {
      font-size: 12px;
      color: ${({ theme }) => theme.colors.textTertiary};
      text-align: center;
    }

    & > strong {
      display: flex;
      align-items: center;
      justify-content: center;
      margin-top: 8px;
    }
  }

  p.message {
    margin: 8px 0 0;
    color: ${({ theme }) => theme.colors.error};
  }
`;

export const Tasks = styled.div`
  position: relative;
  padding-right: 36px;

  .inner {
    overflow: auto;
  }

  .collapse-control {
    position: absolute;
    right: 0;
    top: 0;
  }
`;

export const TasksHeader = styled.ul`
  display: flex;
  align-items: center;

  li {
    display: flex;
    justify-content: space-between;
    align-items: center;
    flex: 0 0 30%;
    padding: 8px 12px;

    &.ready,
    &.cancel {
      color: ${({ theme }) => theme.colors.textTertiary};
      background-color: ${({ theme }) => theme.colors.bgStatus};
    }

    &.loading {
      color: ${({ theme }) => theme.colors.primary};
      background-color: ${({ theme }) => theme.colors.infoBg};
    }

    &.success {
      color: ${({ theme }) => theme.colors.success};
      background-color: ${({ theme }) => theme.colors.successBg};
    }

    &.error {
      color: ${({ theme }) => theme.colors.errorAlt};
      background-color: ${({ theme }) => theme.colors.errorBg};
    }
  }

  li + li {
    margin-left: 16px;
  }
`;

export const TasksList = styled.ul`
  display: flex;
  align-items: flex-start;

  li {
    flex: 0 0 30%;
    padding-bottom: 8px;
    overflow: hidden;
  }

  li + li {
    margin-left: 16px;
  }
`;

export const Task = styled.div`
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 0;
  height: 80px;
  border-bottom: 1px solid ${({ theme }) => theme.colors.borderTask};
  box-sizing: border-box;

  .info {
    flex: auto;
    overflow: hidden;

    .title {
      display: flex;
      align-items: center;
      margin-bottom: 8px;

      & > img {
        width: 20px;
      }

      & > strong {
        margin: 0 4px;
      }

      & > span {
        flex: auto;
        overflow: hidden;
      }
    }

    p {
      padding-left: 26px;
      margin: 0;
      font-size: 12px;
      overflow: hidden;
      white-space: nowrap;
      text-overflow: ellipsis;

      &.error {
        color: ${({ theme }) => theme.colors.error};
      }
    }
  }

  .duration {
    display: flex;
    flex-direction: column;
    align-items: center;
    flex: 0 0 80px;
    text-align: right;
  }
`;
