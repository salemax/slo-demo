import useAsync from 'react-use/esm/useAsync';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  Typography,
} from '@material-ui/core';
import {
  InfoCard,
  Progress,
  ResponseErrorPanel,
} from '@backstage/core-components';
import {
  configApiRef,
  discoveryApiRef,
  fetchApiRef,
  useApi,
} from '@backstage/core-plugin-api';
import { useEntity } from '@backstage/plugin-catalog-react';
import {
  GRAFANA_DASHBOARD_UID_ANNOTATION,
  SLO_ANNOTATION,
  sloNamesOf,
} from '../../annotations';
import { fetchSloValues } from '../../api/prometheus';
import {
  NO_DATA,
  complianceOf,
  formatBudgetAsPercent,
  formatBurnRate,
  formatRatioAsPercent,
} from './formatters';

/**
 * SLO summary for one catalog entity: target, 28-day compliance, error
 * budget remaining and the current (5m) burn rate, one row per `slo`
 * label the entity's annotation lists.
 *
 * The card reads recording rules through the Backstage proxy (D8) and
 * does no SLI arithmetic of its own beyond `1 - error ratio`, which is
 * the same conversion the Grafana dashboard does.
 *
 * It does not poll: the values are fetched when the page mounts. A
 * refresh interval would be a placeholder, and the dashboard is the right
 * place to watch a burn live.
 */
export const SloCard = () => {
  const { entity } = useEntity();
  const discoveryApi = useApi(discoveryApiRef);
  const fetchApi = useApi(fetchApiRef);
  const configApi = useApi(configApiRef);

  const sloNames = sloNamesOf(entity);
  const dashboardUid =
    entity.metadata.annotations?.[GRAFANA_DASHBOARD_UID_ANNOTATION];
  // Placeholder config key, local-dev value only; absent means no link.
  const grafanaBaseUrl = configApi.getOptionalString('slo.grafanaBaseUrl');
  const deepLink =
    dashboardUid && grafanaBaseUrl
      ? {
          title: 'Open the SLO dashboard',
          link: `${grafanaBaseUrl.replace(/\/+$/, '')}/d/${dashboardUid}`,
        }
      : undefined;

  const { value, loading, error } = useAsync(
    () => fetchSloValues({ discoveryApi, fetchApi }),
    [discoveryApi, fetchApi],
  );

  if (loading) {
    return (
      <InfoCard title="SLO">
        <Progress />
      </InfoCard>
    );
  }

  // An unreachable Prometheus must not look like a healthy service, so the
  // error is shown instead of the table.
  if (error) {
    return (
      <InfoCard title="SLO">
        <ResponseErrorPanel error={error} />
      </InfoCard>
    );
  }

  if (sloNames.length === 0) {
    return (
      <InfoCard title="SLO">
        <Typography variant="body2">
          {`This entity has no ${SLO_ANNOTATION} annotation.`}
        </Typography>
      </InfoCard>
    );
  }

  return (
    <InfoCard title="SLO" subheader="28-day window" deepLink={deepLink}>
      <Table size="small">
        <TableHead>
          <TableRow>
            <TableCell>SLO</TableCell>
            <TableCell>Target</TableCell>
            <TableCell>Compliance</TableCell>
            <TableCell>Budget left</TableCell>
            <TableCell>Burn rate (5m)</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {sloNames.map(name => {
            const values = value?.[name];
            return (
              <TableRow key={name} data-testid={`slo-row-${name}`}>
                <TableCell>{name}</TableCell>
                <TableCell>{formatRatioAsPercent(values?.objective)}</TableCell>
                <TableCell>
                  {formatRatioAsPercent(complianceOf(values?.sliError28d))}
                </TableCell>
                <TableCell>
                  {formatBudgetAsPercent(values?.budgetRemaining28d)}
                </TableCell>
                <TableCell>{formatBurnRate(values?.burnRate5m)}</TableCell>
              </TableRow>
            );
          })}
        </TableBody>
      </Table>
      <Typography variant="caption">
        {`${NO_DATA} means the series is missing or NaN: an idle service records no requests, so its ratios are 0/0.`}
      </Typography>
    </InfoCard>
  );
};
