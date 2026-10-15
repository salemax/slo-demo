import { DiscoveryApi, FetchApi } from '@backstage/core-plugin-api';
import { SLO_QUERY, fetchSloValues } from './prometheus';

const discoveryApi: DiscoveryApi = {
  getBaseUrl: async (id: string) => `http://localhost:7007/api/${id}`,
};

function fetchApiReturning(
  body: unknown,
  init?: { ok?: boolean; status?: number; statusText?: string },
): { fetchApi: FetchApi; urls: string[] } {
  const urls: string[] = [];
  const fetchApi = {
    fetch: async (input: RequestInfo | URL) => {
      urls.push(String(input));
      return {
        ok: init?.ok ?? true,
        status: init?.status ?? 200,
        statusText: init?.statusText ?? 'OK',
        json: async () => body,
      } as Response;
    },
  } as FetchApi;
  return { fetchApi, urls };
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
        value: [1760000000, value] as [number, string],
      })),
    },
  };
}

describe('fetchSloValues', () => {
  it('asks the proxy, not Prometheus directly, and queries only recording rules', async () => {
    const { fetchApi, urls } = fetchApiReturning(instantResponse([]));

    await fetchSloValues({ discoveryApi, fetchApi });

    expect(urls).toHaveLength(1);
    const url = new URL(urls[0]);
    expect(url.pathname).toBe('/api/proxy/prometheus/api/query');
    expect(url.searchParams.get('query')).toBe(SLO_QUERY);
    // Every name in the query is a slo:* recording rule.
    expect(SLO_QUERY).toMatch(/^\{__name__=~"slo:[^"]*"\}$/);
    expect(SLO_QUERY).not.toMatch(/http_requests_total|http_request_duration/);
  });

  it('groups the series by the slo label', async () => {
    const { fetchApi } = fetchApiReturning(
      instantResponse([
        ['slo:objective:ratio', 'availability', '0.995'],
        ['slo:objective:ratio', 'latency', '0.95'],
        ['slo:sli_error:ratio_rate28d', 'availability', '0.0005'],
        ['slo:error_budget_remaining:ratio_rate28d', 'availability', '0.9'],
        ['slo:error_budget_burn_rate:ratio_rate5m', 'availability', '8.6'],
      ]),
    );

    const values = await fetchSloValues({ discoveryApi, fetchApi });

    expect(values.availability).toEqual({
      objective: 0.995,
      sliError28d: 0.0005,
      budgetRemaining28d: 0.9,
      burnRate5m: 8.6,
    });
    expect(values.latency).toEqual({ objective: 0.95 });
  });

  it('keeps an idle service NaN instead of turning it into 0', async () => {
    const { fetchApi } = fetchApiReturning(
      instantResponse([['slo:sli_error:ratio_rate28d', 'availability', 'NaN']]),
    );

    const values = await fetchSloValues({ discoveryApi, fetchApi });

    expect(values.availability.sliError28d).toBeNaN();
  });

  it('ignores series it does not know and series without an slo label', async () => {
    const { fetchApi } = fetchApiReturning({
      status: 'success',
      data: {
        resultType: 'vector',
        result: [
          {
            metric: { __name__: 'slo:objective:ratio' },
            value: [1, '0.995'] as [number, string],
          },
          {
            metric: { __name__: 'up', slo: 'availability' },
            value: [1, '1'] as [number, string],
          },
        ],
      },
    });

    await expect(fetchSloValues({ discoveryApi, fetchApi })).resolves.toEqual(
      {},
    );
  });

  it('fails loudly on an HTTP error, so the card cannot look healthy', async () => {
    const { fetchApi } = fetchApiReturning(
      {},
      { ok: false, status: 503, statusText: 'Service Unavailable' },
    );

    await expect(fetchSloValues({ discoveryApi, fetchApi })).rejects.toThrow(
      'Prometheus query failed: 503 Service Unavailable',
    );
  });

  it('fails loudly when Prometheus reports an error status', async () => {
    const { fetchApi } = fetchApiReturning({
      status: 'error',
      error: 'parse error',
    });

    await expect(fetchSloValues({ discoveryApi, fetchApi })).rejects.toThrow(
      'Prometheus query failed: parse error',
    );
  });
});
