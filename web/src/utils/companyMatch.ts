export function normalizedCompanyName(value: string): string {
  return value
    .normalize("NFKD")
    .replace(/[\u0300-\u036f]/g, "")
    .toLocaleLowerCase()
    .trim();
}

// Suggest a near-matching company while avoiding unrelated short fragments.
export function matchesCompanyName(input: string, candidate: string): boolean {
  const needle = normalizedCompanyName(input);
  const haystack = normalizedCompanyName(candidate);
  if (!needle || haystack.includes(needle)) return true;
  if (needle.length < 4) return false;
  return haystack.split(/\s+/).some((word) => {
    if (Math.abs(word.length - needle.length) > 2) return false;
    const distance = Array.from({ length: needle.length + 1 }, (_, i) => i);
    for (let j = 1; j <= word.length; j++) {
      let previous = distance[0];
      distance[0] = j;
      for (let i = 1; i <= needle.length; i++) {
        const old = distance[i];
        distance[i] = Math.min(
          distance[i] + 1,
          distance[i - 1] + 1,
          previous + (needle[i - 1] === word[j - 1] ? 0 : 1),
        );
        previous = old;
      }
    }
    return distance[needle.length] <= (needle.length >= 7 ? 2 : 1);
  });
}
