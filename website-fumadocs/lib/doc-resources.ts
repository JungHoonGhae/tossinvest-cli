// Keep existing Korean resource URLs; English uses an explicit resource prefix.
export function resourceSegments(locale: string | undefined, slugs: string[], file: string): string[] {
  return [...(locale === 'en' ? ['en'] : []), ...slugs, file];
}

export function parseResourceSegments(segments: string[] | undefined, file: string) {
  if (!segments?.length || segments.at(-1) !== file) return undefined;
  const locale = segments[0] === 'en' ? 'en' : 'ko';
  return { locale, slugs: segments.slice(locale === 'en' ? 1 : 0, -1) };
}
