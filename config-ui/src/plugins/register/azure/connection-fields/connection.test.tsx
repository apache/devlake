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

import { useState, type ReactNode } from 'react';
import '@testing-library/jest-dom/vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { Form } from '@/plugins/components/connection-form/fields';

import { AzureGoConfig } from '../config';

vi.mock('@/components', () => ({
  Block: ({ title, children }: { title?: ReactNode; children?: ReactNode }) => (
    <section>
      {title}
      {children}
    </section>
  ),
  ExternalLink: ({ children }: { children?: ReactNode }) => <span>{children}</span>,
}));

const endpoint = 'https://ado.example.com/tfs/DefaultCollection/';
const initialConnection = { name: 'Test', token: 'test-token', endpoint };

function ConnectionFields({
  initialValues,
  fields = AzureGoConfig.connection.fields,
}: {
  initialValues: Record<string, string>;
  fields?: typeof AzureGoConfig.connection.fields;
}) {
  const [values, setValues] = useState<Record<string, unknown>>({});
  const [errors, setErrors] = useState<Record<string, string>>({});
  return (
    <>
      <Form
        type="update"
        name="Azure DevOps Go"
        fields={fields}
        initialValues={initialValues}
        values={values}
        errors={errors}
        setValues={setValues}
        setErrors={setErrors}
      />
      <button disabled={Object.values(errors).some(Boolean)}>Save</button>
      <output data-testid="values">{JSON.stringify(values)}</output>
    </>
  );
}

describe('Azure DevOps Go connection fields', () => {
  it('allows an empty username for on-premises and Cloud connections', async () => {
    const { rerender } = render(<ConnectionFields initialValues={initialConnection} />);
    await waitFor(() => expect(screen.getByRole('button', { name: 'Save' })).not.toBeDisabled());
    expect(screen.getByPlaceholderText('Your Username')).toHaveValue('');

    rerender(<ConnectionFields initialValues={{ ...initialConnection, endpoint: '' }} />);
    await waitFor(() => expect(screen.getByRole('radio', { name: 'Azure DevOps Cloud' })).toBeChecked());
    expect(screen.getByRole('button', { name: 'Save' })).not.toBeDisabled();
  });

  it('restores a saved on-premises endpoint even when the connection loads after mount', async () => {
    const { rerender } = render(<ConnectionFields initialValues={{}} />);
    rerender(<ConnectionFields initialValues={initialConnection} />);

    await waitFor(() => expect(screen.getByRole('radio', { name: 'Azure DevOps Server (On-Premises)' })).toBeChecked());
    expect(screen.getByPlaceholderText('e.g., https://your-server/tfs/DefaultCollection/')).toHaveValue(endpoint);
    expect(JSON.parse(screen.getByTestId('values').textContent ?? '{}').endpoint).toBe(endpoint);

    const updatedEndpoint = 'https://other.example.com/tfs/Collection/';
    fireEvent.change(screen.getByPlaceholderText('e.g., https://your-server/tfs/DefaultCollection/'), {
      target: { value: updatedEndpoint },
    });
    fireEvent.change(screen.getByPlaceholderText('Your Username'), { target: { value: 'user' } });
    expect(JSON.parse(screen.getByTestId('values').textContent ?? '{}').endpoint).toBe(updatedEndpoint);
  });

  it('clears the endpoint when switching to Cloud and requires a URL when switching back', async () => {
    render(<ConnectionFields initialValues={initialConnection} />);
    fireEvent.click(await screen.findByRole('radio', { name: 'Azure DevOps Cloud' }));
    await waitFor(() => expect(JSON.parse(screen.getByTestId('values').textContent ?? '{}').endpoint).toBe(''));
    fireEvent.click(screen.getByRole('radio', { name: 'Azure DevOps Server (On-Premises)' }));
    await waitFor(() => expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled());
    fireEvent.change(screen.getByPlaceholderText('e.g., https://your-server/tfs/DefaultCollection/'), {
      target: { value: endpoint },
    });
    await waitFor(() => expect(screen.getByRole('button', { name: 'Save' })).not.toBeDisabled());
  });

  it('keeps username required by default for other connections', async () => {
    render(<ConnectionFields initialValues={{}} fields={['username']} />);
    await waitFor(() => expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled());
    fireEvent.change(screen.getByPlaceholderText('Your Username'), { target: { value: 'user' } });
    await waitFor(() => expect(screen.getByRole('button', { name: 'Save' })).not.toBeDisabled());
  });
});
