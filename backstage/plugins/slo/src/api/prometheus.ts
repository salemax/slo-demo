import { DiscoveryApi, FetchApi } from '@backstage/core-plugin-api';

/**
 * The recording rules the card reads. It reads only rules, never raw
 * metrics: the SLI, budget and burn-rate arithmetic lives in
 * monitoring/prometheus/rules/slo-recording.yml and must exist in one
 * place only (D1-D6 in docs/PLAN.md).
 */
export const SLO_METRICS = {
  objective: 'slo:objective:ratio',
  sliError28d: 'slo:sli_error:ratio_rate28d',
  budgetRemaining28d: 'slo:error_budget_remaining:ratio_rate28d',
  burnRate5m: 'slo:error_budget_burn_rate:ratio_rate5m',
} as const;

export type SloMetric = keyof typeof SLO_METRICS;

/** One row of the card: metric -> value, for a single `slo` label value. */
export type SloValues = Partial<Record<SloMetric, number>>;

/** Keyed by the `slo` label, e.g. "availability" and "latency". */
export type SloValuesBySlo = Record<string, SloValues>;

type PrometheusInstantResponse = {
  status: 'success' | 'error';
  error?: string;
  data?: {
    resultType: string;
    result?: Array<{
      metric: Record<string, string>;
      value: [number, string];
    }>;
  };
};

const METRIC_BY_NAME: Record<string, SloMetric> = Object.fromEntries(
  Object.entries(SLO_METRICS).map(([key, name]) => [name, key as SloMetric]),
) as Record<string, SloMetric>;

/**
 * All four series in one instant query. A single request keeps the card's
 * numbers from the same evaluation as each other, the way the recording
 * rules intend.
 */
export const SLO_QUERY = `{__name__=~"${Object.values(SLO_METRICS).join(
  '|',
)}"}`;

export async function fetchSloValues(options: {
  discoveryApi: DiscoveryApi;
  fetchApi: FetchApi;
}): Promise<SloValuesBySlo> {
  const { discoveryApi, fetchApi } = options;

  // The proxy endpoint is configured in backstage/app-config.yaml (D8):
  // it is the only place that knows where Prometheus is.
  const proxyBaseUrl = await discoveryApi.getBaseUrl('proxy');
  const url = `${proxyBaseUrl}/prometheus/api/query?query=${encodeURIComponent(
    SLO_QUERY,
  )}`;

  const response = await fetchApi.fetch(url);
  if (!response.ok) {
    throw new Error(
      `Prometheus query failed: ${response.status} ${response.statusText}`,
    );
  }

  const body = (await response.json()) as PrometheusInstantResponse;
  if (body.status !== 'success') {
    throw new Error(
      `Prometheus query failed: ${body.error ?? 'unknown error'}`,
    );
  }

  const bySlo: SloValuesBySlo = {};
  for (const series of body.data?.result ?? []) {
    const metric = METRIC_BY_NAME[series.metric.__name__];
    const slo = series.metric.slo;
    if (!metric || !slo) {
      continue;
    }
    // Prometheus renders an idle service's 0/0 ratio as the string "NaN".
    // Number("NaN") is NaN, and the card renders that as "No data" rather
    // than as a healthy 0 (the same rule the Grafana dashboard follows).
    bySlo[slo] = { ...bySlo[slo], [metric]: Number(series.value[1]) };
  }
  return bySlo;
}
