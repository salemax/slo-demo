import { Entity } from '@backstage/catalog-model';
import {
  configApiRef,
  discoveryApiRef,
  fetchApiRef,
} from '@backstage/core-plugin-api';
import { EntityProvider } from '@backstage/plugin-catalog-react';
import {
  TestApiProvider,
  mockApis,
  renderInTestApp,
} from '@backstage/frontend-test-utils';
import { screen, within } from '@testing-library/react';
import { SLO_ANNOTATION } from '../../annotations';
import { SloCard } from './SloCard';

const entity: Entity = {
  apiVersion: 'backstage.io/v1alpha1',
  kind: 'Component',
  metadata: {
    name: 'slo-demo-service',
    annotations: {
      [SLO_ANNOTATION]: 'availability,latency',
      'slo-demo/grafana-dashboard-uid': 'slo-demo',
    },
  },
};

const discoveryApi = {
  getBaseUrl: async (id: string) => `http://localhost:7007/api/${id}`,
};

function fetchApiReturning(
  body: unknown,
  init?: { ok?: boolean; status?: number; statusText?: string },
) {
  return {
    fetch: async () =>
      ({
        ok: init?.ok ?? true,
        status: init?.status ?? 200,
        statusText: init?.statusText ?? 'OK',
        json: async () => body,
      } as Response),
  };
}

function instantResponse(
  series: Array<[name: string, slo: string, value: string]>,
) {
  return {
    status: 'success',
    data: {
      resultType: 'vector',
      result: series.map(([name, slo, value]) => ({
        metric: { __name__: name, slo },
        value: [1760000000, value],
      })),
    },
  };
}

async function renderCard(
  body: unknown,
  options?: {
    init?: { ok?: boolean; status?: number; statusText?: string };
    entity?: Entity;
    config?: object;
  },
) {
  await renderInTestApp(
    <TestApiProvider
      apis={[
        [discoveryApiRef, discoveryApi],
        [fetchApiRef, fetchApiReturning(body, options?.init)],
        [
          configApiRef,
          mockApis.config({
            data: options?.config ?? {
              slo: { grafanaBaseUrl: 'http://localhost:3000' },
            },
          }),
        ],
      ]}
    >
      <EntityProvider entity={options?.entity ?? entity}>
        <SloCard />
      </EntityProvider>
    </TestApiProvider>,
  );
}

function row(slo: string) {
  return within(screen.getByTestId(`slo-row-${slo}`));
}

describe('SloCard', () => {
  it('shows one row per slo label, with target, compliance, budget and burn rate', async () => {
    await renderCard(
      instantResponse([
        ['slo:objective:ratio', 'availability', '0.995'],
        ['slo:sli_error:ratio_rate28d', 'availability', '0.0005'],
        ['slo:error_budget_remaining:ratio_rate28d', 'availability', '0.9'],
        ['slo:error_budget_burn_rate:ratio_rate5m', 'availability', '8.6'],
        ['slo:objective:ratio', 'latency', '0.95'],
        ['slo:sli_error:ratio_rate28d', 'latency', '0.04'],
        ['slo:error_budget_remaining:ratio_rate28d', 'latency', '0.2'],
        ['slo:error_budget_burn_rate:ratio_rate5m', 'latency', '1.07'],
      ]),
    );

    expect(await screen.findByText('availability')).toBeInTheDocument();
    expect(row('availability').getByText('99.500%')).toBeInTheDocument();
    expect(row('availability').getByText('99.950%')).toBeInTheDocument();
    expect(row('availability').getByText('90.0%')).toBeInTheDocument();
    expect(row('availability').getByText('8.60x')).toBeInTheDocument();

    expect(row('latency').getByText('95.000%')).toBeInTheDocument();
    expect(row('latency').getByText('96.000%')).toBeInTheDocument();
    expect(row('latency').getByText('20.0%')).toBeInTheDocument();
    expect(row('latency').getByText('1.07x')).toBeInTheDocument();
  });

  it('shows NaN as no data, not as a healthy zero', async () => {
    await renderCard(
      instantResponse([
        ['slo:objective:ratio', 'availability', '0.995'],
        ['slo:sli_error:ratio_rate28d', 'availability', 'NaN'],
        ['slo:error_budget_remaining:ratio_rate28d', 'availability', 'NaN'],
        ['slo:error_budget_burn_rate:ratio_rate5m', 'availability', 'NaN'],
      ]),
    );

    expect(await screen.findByText('availability')).toBeInTheDocument();
    const availability = row('availability');
    expect(availability.getAllByText('No data')).toHaveLength(3);
    expect(availability.queryByText('0.00x')).not.toBeInTheDocument();
    expect(availability.queryByText('0.0%')).not.toBeInTheDocument();
  });

  it('shows no data for an slo the rules return nothing for', async () => {
    await renderCard(
      instantResponse([['slo:objective:ratio', 'availability', '0.995']]),
    );

    expect(await screen.findByText('latency')).toBeInTheDocument();
    expect(row('latency').getAllByText('No data')).toHaveLength(4);
  });

  it('shows the error instead of the table when Prometheus is unreachable', async () => {
    await renderCard(
      {},
      { init: { ok: false, status: 503, statusText: 'Service Unavailable' } },
    );

    // ResponseErrorPanel repeats the message in its summary and its details.
    expect(
      await screen.findAllByText(/503 Service Unavailable/),
    ).not.toHaveLength(0);
    expect(
      screen.queryByTestId('slo-row-availability'),
    ).not.toBeInTheDocument();
  });

  it('links to the provisioned Grafana dashboard', async () => {
    await renderCard(instantResponse([]));

    const link = await screen.findByRole('link', {
      name: /Open the SLO dashboard/i,
    });
    expect(link).toHaveAttribute('href', 'http://localhost:3000/d/slo-demo');
  });

  it('omits the link when no Grafana base url is configured', async () => {
    await renderCard(instantResponse([]), { config: {} });

    expect(await screen.findByText('availability')).toBeInTheDocument();
    expect(
      screen.queryByRole('link', { name: /Open the SLO dashboard/i }),
    ).not.toBeInTheDocument();
  });

  it('says so when the entity carries no slo annotation', async () => {
    await renderCard(instantResponse([]), {
      entity: {
        apiVersion: 'backstage.io/v1alpha1',
        kind: 'Component',
        metadata: { name: 'other' },
      },
    });

    expect(
      await screen.findByText(
        `This entity has no ${SLO_ANNOTATION} annotation.`,
      ),
    ).toBeInTheDocument();
  });
});
