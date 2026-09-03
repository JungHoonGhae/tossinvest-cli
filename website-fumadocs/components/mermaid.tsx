'use client';

import { useEffect, useId, useRef, useState } from 'react';
import { useTheme } from 'next-themes';
import type { RenderResult } from 'mermaid';

export function Mermaid({ chart }: { chart: string }) {
  const reactId = useId();
  const id = `mermaid-${reactId.replaceAll(':', '')}`;
  const { resolvedTheme } = useTheme();
  const containerRef = useRef<HTMLDivElement>(null);
  const [result, setResult] = useState<RenderResult>();
  const [error, setError] = useState<string>();

  useEffect(() => {
    let cancelled = false;

    async function render() {
      try {
        const { default: mermaid } = await import('mermaid');
        mermaid.initialize({
          startOnLoad: false,
          securityLevel: 'strict',
          fontFamily: 'inherit',
          theme: resolvedTheme === 'dark' ? 'dark' : 'default',
        });
        const next = await mermaid.render(id, chart.replaceAll('\\n', '\n'));
        if (!cancelled) {
          setResult(next);
          setError(undefined);
        }
      } catch (cause) {
        if (!cancelled) {
          setResult(undefined);
          setError(cause instanceof Error ? cause.message : String(cause));
        }
      }
    }

    void render();
    return () => {
      cancelled = true;
    };
  }, [chart, id, resolvedTheme]);

  useEffect(() => {
    if (result && containerRef.current) result.bindFunctions?.(containerRef.current);
  }, [result]);

  if (error) {
    return (
      <figure className="my-6 overflow-x-auto rounded-xl border border-fd-destructive/30 bg-fd-card p-4">
        <figcaption className="mb-2 text-sm text-fd-destructive">
          Diagram could not be rendered: {error}
        </figcaption>
        <pre className="text-xs">{chart}</pre>
      </figure>
    );
  }

  return (
    <div
      aria-busy={!result}
      aria-label="Diagram"
      className="my-6 min-h-24 overflow-x-auto rounded-xl border bg-fd-card p-4 [&_svg]:mx-auto [&_svg]:h-auto [&_svg]:max-w-full"
      ref={containerRef}
      dangerouslySetInnerHTML={result ? { __html: result.svg } : undefined}
    />
  );
}
