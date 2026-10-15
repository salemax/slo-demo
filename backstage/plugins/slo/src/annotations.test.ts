import { Entity } from '@backstage/catalog-model';
import { SLO_ANNOTATION, hasSlos, sloNamesOf } from './annotations';

function entityWith(annotations?: Record<string, string>): Entity {
  return {
    apiVersion: 'backstage.io/v1alpha1',
    kind: 'Component',
    metadata: { name: 'slo-demo-service', annotations },
  };
}

describe('sloNamesOf', () => {
  it('splits and trims the annotation value', () => {
    const entity = entityWith({
      [SLO_ANNOTATION]: 'availability, latency',
    });
    expect(sloNamesOf(entity)).toEqual(['availability', 'latency']);
  });

  it('keeps the annotation order, because it is the row order', () => {
    const entity = entityWith({ [SLO_ANNOTATION]: 'latency,availability' });
    expect(sloNamesOf(entity)).toEqual(['latency', 'availability']);
  });

  it('drops empty entries', () => {
    const entity = entityWith({ [SLO_ANNOTATION]: 'availability,,  ,' });
    expect(sloNamesOf(entity)).toEqual(['availability']);
  });

  it('returns nothing when the annotation is absent', () => {
    expect(sloNamesOf(entityWith())).toEqual([]);
    expect(sloNamesOf(entityWith({ 'other/annotation': 'x' }))).toEqual([]);
  });
});

describe('hasSlos', () => {
  it('is true only for an entity with at least one slo name', () => {
    expect(hasSlos(entityWith({ [SLO_ANNOTATION]: 'availability' }))).toBe(
      true,
    );
    expect(hasSlos(entityWith())).toBe(false);
    // Present but empty: no card, because an empty card is worse than none.
    expect(hasSlos(entityWith({ [SLO_ANNOTATION]: '  ' }))).toBe(false);
  });
});
