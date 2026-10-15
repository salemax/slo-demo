import {
  NO_DATA,
  complianceOf,
  formatBudgetAsPercent,
  formatBurnRate,
  formatRatioAsPercent,
} from './formatters';

describe('formatters', () => {
  it('formats ratios as percentages', () => {
    expect(formatRatioAsPercent(0.995)).toBe('99.500%');
    expect(formatRatioAsPercent(0.9995)).toBe('99.950%');
  });

  it('shows NaN and missing values as no data, never as zero', () => {
    for (const format of [
      formatRatioAsPercent,
      formatBudgetAsPercent,
      formatBurnRate,
    ]) {
      expect(format(NaN)).toBe(NO_DATA);
      expect(format(undefined)).toBe(NO_DATA);
      expect(format(Infinity)).toBe(NO_DATA);
    }
  });

  it('keeps a spent budget negative', () => {
    expect(formatBudgetAsPercent(-20.4)).toBe('-2040.0%');
    expect(formatBudgetAsPercent(0)).toBe('0.0%');
  });

  it('formats burn rate as a multiplier', () => {
    expect(formatBurnRate(14.4)).toBe('14.40x');
    expect(formatBurnRate(0)).toBe('0.00x');
  });

  it('turns an error ratio into compliance', () => {
    expect(complianceOf(0.005)).toBeCloseTo(0.995);
    expect(complianceOf(NaN)).toBeUndefined();
    expect(complianceOf(undefined)).toBeUndefined();
  });
});
