/**
 * Language-independent facts of the Life Path pages: which numbers have a page,
 * a real birth date that reduces to each, and the traditionally compatible numbers.
 */
export const LIFE_PATH_NUMBERS = [1, 2, 3, 4, 5, 6, 7, 8, 9, 11, 22, 33] as const;
export type LifePathNumber = (typeof LIFE_PATH_NUMBERS)[number];

export function isLifePathNumber(value: number): value is LifePathNumber {
  return (LIFE_PATH_NUMBERS as readonly number[]).includes(value);
}

/** The page slug of a number: "7", "11". */
export function lifePathSlug(number: number): string {
  return String(number);
}

/** The number of a slug ("07", "x", "13" → null). */
export function parseLifePathSlug(slug: string): LifePathNumber | null {
  if (!/^[1-9]\d?$/.test(slug)) return null;
  const value = Number(slug);
  return isLifePathNumber(value) ? value : null;
}

/** A real birth date (YYYY-MM-DD) whose Life Path is the number (checked by the tests). */
export const LIFE_PATH_EXAMPLE_DATE: Record<LifePathNumber, string> = {
  1: "1992-01-15",
  2: "1988-05-25",
  3: "1989-02-28",
  4: "1986-10-15",
  5: "1986-09-17",
  6: "1983-05-25",
  7: "1981-06-18",
  8: "1979-08-10",
  9: "1976-09-22",
  11: "1972-09-10",
  22: "1980-05-17",
  33: "1975-04-16",
};

/** The single digit a master number reduces to when it is not kept (11 → 2, 22 → 4, 33 → 6). */
export const MASTER_ROOT = { 11: 2, 22: 4, 33: 6 } as const;

/**
 * Numbers that numerologists traditionally pair with each Life Path. Schools
 * differ; the pages present these as a starting point for reflection.
 */
export const LIFE_PATH_COMPATIBLE: Record<LifePathNumber, readonly LifePathNumber[]> = {
  1: [3, 5],
  2: [4, 6, 8],
  3: [1, 5, 9],
  4: [2, 6, 8],
  5: [1, 3, 7],
  6: [2, 4, 8, 9],
  7: [5, 9],
  8: [2, 4, 6],
  9: [3, 6, 7],
  11: [2, 6, 22],
  22: [4, 8, 11],
  33: [6, 9, 11],
};

/** Neighbouring numbers for the related links (previous and next in the list). */
export function lifePathNeighbours(number: LifePathNumber): LifePathNumber[] {
  const index = LIFE_PATH_NUMBERS.indexOf(number);
  return [LIFE_PATH_NUMBERS[index - 1], LIFE_PATH_NUMBERS[index + 1]].filter((n): n is LifePathNumber => n !== undefined);
}
