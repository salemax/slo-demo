import { Entity } from '@backstage/catalog-model';

/**
 * Presence of this annotation is what makes the SLO card appear on an
 * entity page; its value lists the `slo` label values to show as rows.
 * The values must match the `slo` label on the slo:* recording rules in
 * monitoring/prometheus/rules/slo-recording.yml.
 */
export const SLO_ANNOTATION = 'slo-demo/slos';

/** uid of the provisioned Grafana dashboard, used for the card's deep link. */
export const GRAFANA_DASHBOARD_UID_ANNOTATION =
  'slo-demo/grafana-dashboard-uid';

/** The `slo` label values an entity declares, in annotation order. */
export function sloNamesOf(entity: Entity): string[] {
  return (entity.metadata.annotations?.[SLO_ANNOTATION] ?? '')
    .split(',')
    .map(name => name.trim())
    .filter(Boolean);
}

/**
 * Card filter. An entity with the annotation but no usable value gets no
 * card, because an empty card is worse than no card.
 */
export function hasSlos(entity: Entity): boolean {
  return sloNamesOf(entity).length > 0;
}
