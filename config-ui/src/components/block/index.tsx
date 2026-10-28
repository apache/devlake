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

const Wrapper = styled.div`
  & > h4 {
    display: flex;

    & > i {
      margin-left: 4px;
      color: #ff8b8b;
    }
  }

  & > p {
    margin-top: 5px;
  }

  & + & {
    margin-top: 16px;
  }
`;

interface Props {
  style?: React.CSSProperties;
  title?: React.ReactNode;
  description?: React.ReactNode;
  required?: boolean;
  error?: string;
  children?: React.ReactNode;
}

export const Block = ({ style, title, description, required, error, children }: Props) => {
  return (
    <Wrapper style={style}>
      {title && (
        <h4>
          {title}
          {required && <i>*</i>}
        </h4>
      )}
      {description && <p>{description}</p>}
      {children}
      {error && <p style={{ color: '#ff4d4f', marginTop: 4 }}>{error}</p>}
    </Wrapper>
  );
};
