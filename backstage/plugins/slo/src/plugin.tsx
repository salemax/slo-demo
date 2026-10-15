import { createFrontendPlugin } from '@backstage/frontend-plugin-api';
import { EntityCardBlueprint } from '@backstage/plugin-catalog-react/alpha';
import { hasSlos } from './annotations';

/**
 * Entity card showing SLO target, compliance, budget and burn rate.
 *
 * `filter` is a function rather than a filter expression so that the
 * annotation key, which contains a slash and is parsed in one place,
 * stays in TypeScript.
 *
 * `type: 'content'` rather than 'info': an info card sits in the narrow
 * right-hand column (measured 330 px), where the five columns wrap onto
 * three lines each. In the content area the same table is 661 px wide and
 * fits on one line.
 */
export const sloEntityCard = EntityCardBlueprint.make({
  name: 'slo',
  params: {
    type: 'content',
    filter: hasSlos,
    loader: () => import('./components/SloCard').then(m => <m.SloCard />),
  },
});

export const sloPlugin = createFrontendPlugin({
  pluginId: 'slo',
  extensions: [sloEntityCard],
});
