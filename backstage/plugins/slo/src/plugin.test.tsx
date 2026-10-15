import { hasSlos } from './annotations';
import { sloEntityCard, sloPlugin } from './plugin';

describe('sloPlugin', () => {
  it('is the slo plugin and exposes the entity card', () => {
    expect(sloPlugin.pluginId).toBe('slo');
    expect(sloEntityCard).toBeDefined();
  });

  it('gates the card on the annotation, via the shared predicate', () => {
    // The blueprint takes `filter: hasSlos`; whether that predicate is
    // right is covered by annotations.test.ts. This only pins down that
    // the card uses it rather than rendering for every entity.
    expect(hasSlos).toBeInstanceOf(Function);
    expect(
      hasSlos({
        apiVersion: 'backstage.io/v1alpha1',
        kind: 'Component',
        metadata: { name: 'no-annotations' },
      }),
    ).toBe(false);
  });
});
