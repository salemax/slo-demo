/**
 * Formatting only. Every choice here (decimal places, the "No data" text,
 * the x suffix) is a placeholder, not a decision from docs/PLAN.md.
 */
export const NO_DATA = 'No data';

function isMissing(value: number | undefined): boolean {
  return value === undefined || !Number.isFinite(value);
}

/** 0.995 -> "99.500%". Keeps three decimals so 99.5% and 99.95% differ. */
export function formatRatioAsPercent(value: number | undefined): string {
  return isMissing(value)
    ? NO_DATA
    : `${((value as number) * 100).toFixed(3)}%`;
}

/**
 * Budget remaining, which is negative once the budget is spent (a value of
 * -20.4 is -2040%, as seen in the #44 review). One decimal: the sign and
 * the magnitude are what matter, not the third digit.
 */
export function formatBudgetAsPercent(value: number | undefined): string {
  return isMissing(value)
    ? NO_DATA
    : `${((value as number) * 100).toFixed(1)}%`;
}

/** Burn rate is unitless: 14.4 means the 28d budget lasts 1/14.4 of 28d. */
export function formatBurnRate(value: number | undefined): string {
  return isMissing(value) ? NO_DATA : `${(value as number).toFixed(2)}x`;
}

/** Compliance is 1 - the error ratio the recording rules record. */
export function complianceOf(sliError: number | undefined): number | undefined {
  return isMissing(sliError) ? undefined : 1 - (sliError as number);
}
