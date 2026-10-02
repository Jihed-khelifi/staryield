export const zodiacSigns = [
  "Capricorn",
  "Aquarius",
  "Pisces",
  "Aries",
  "Taurus",
  "Gemini",
  "Cancer",
  "Leo",
  "Virgo",
  "Libra",
  "Scorpio",
  "Sagittarius",
] as const;
const starts = [20, 19, 21, 20, 21, 21, 23, 23, 23, 23, 22, 22];
export function zodiacForDate(month: number, day: number) {
  return zodiacSigns[day < starts[month - 1] ? month - 1 : month % 12];
}
export function reduceNumber(value: number, masters = true): number {
  while (value > 9 && !(masters && [11, 22, 33].includes(value)))
    value = String(value)
      .split("")
      .reduce((sum, digit) => sum + Number(digit), 0);
  return value;
}
export function lifePath(date: string) {
  return reduceNumber(
    date
      .split("-")
      .map((part) => reduceNumber(Number(part)))
      .reduce((sum, n) => sum + n, 0),
  );
}
export function personalCycles(birth: string, today = new Date()) {
  const [, month, day] = birth.split("-").map(Number);
  const personalYear = reduceNumber(
    month + day + reduceNumber(today.getFullYear(), false),
    false,
  );
  return {
    personalYear,
    personalMonth: reduceNumber(personalYear + today.getMonth() + 1, false),
  };
}
export function minimumAdultDate(today = new Date()) {
  const cutoff = new Date(today);
  cutoff.setFullYear(cutoff.getFullYear() - 18);
  return `${cutoff.getFullYear()}-${String(cutoff.getMonth() + 1).padStart(2, "0")}-${String(cutoff.getDate()).padStart(2, "0")}`;
}
export function validBirthDate(
  day: number,
  month: number,
  year: number,
  today = new Date(),
) {
  const date = new Date(Date.UTC(year, month - 1, day));
  return (
    year >= 1900 &&
    year <= today.getFullYear() &&
    date.getUTCFullYear() === year &&
    date.getUTCMonth() === month - 1 &&
    date.getUTCDate() === day &&
    date <=
      new Date(Date.UTC(today.getFullYear(), today.getMonth(), today.getDate()))
  );
}
