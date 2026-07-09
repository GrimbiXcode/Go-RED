/**
 * Small monochrome SVG icon set for node categories, replacing the emoji
 * fallbacks in the palette (Phase 2 of docs/FRONTEND_NODE_RED_REDESIGN.md).
 * Server-provided `<svg>` node icons (via NodeMetadata.icon) are untouched
 * and still go through DOMPurify — this only covers the built-in fallback.
 */

export interface CategoryIconProps {
  category: string;
  className?: string;
}

const iconProps = {
  viewBox: '0 0 24 24',
  fill: 'none' as const,
  stroke: 'currentColor',
  strokeWidth: 2,
  strokeLinecap: 'round' as const,
  strokeLinejoin: 'round' as const,
};

const icons: Record<string, React.ReactNode> = {
  input: (
    <>
      <path d="M3 12h13" />
      <polyline points="11 6 17 12 11 18" />
      <line x1="21" y1="4" x2="21" y2="20" />
    </>
  ),
  output: (
    <>
      <line x1="3" y1="4" x2="3" y2="20" />
      <path d="M8 12h13" />
      <polyline points="15 6 21 12 15 18" />
    </>
  ),
  function: (
    <>
      <path d="M9 4c-2 0-3 1-3 3v3a2 2 0 0 1-2 2 2 2 0 0 1 2 2v3c0 2 1 3 3 3" />
      <path d="M15 4c2 0 3 1 3 3v3a2 2 0 0 0 2 2 2 2 0 0 0-2 2v3c0 2-1 3-3 3" />
    </>
  ),
  storage: (
    <>
      <ellipse cx="12" cy="6" rx="7" ry="3" />
      <path d="M5 6v12c0 1.7 3.1 3 7 3s7-1.3 7-3V6" />
      <path d="M5 12c0 1.7 3.1 3 7 3s7-1.3 7-3" />
    </>
  ),
  network: (
    <>
      <circle cx="12" cy="12" r="9" />
      <ellipse cx="12" cy="12" rx="4" ry="9" />
      <line x1="3" y1="12" x2="21" y2="12" />
    </>
  ),
  protocol: (
    <>
      <path d="M9 2v4" />
      <path d="M15 2v4" />
      <path d="M7 6h10a2 2 0 0 1 2 2v2a7 7 0 0 1-14 0V8a2 2 0 0 1 2-2z" />
      <path d="M12 17v5" />
    </>
  ),
  parser: (
    <>
      <path d="M6 2h9l5 5v15H6z" />
      <line x1="9" y1="13" x2="16" y2="13" />
      <line x1="9" y1="17" x2="16" y2="17" />
    </>
  ),
  social: <path d="M21 11.5a8.4 8.4 0 0 1-8.4 8.4H12l-5 3 1-4.5A8.4 8.4 0 1 1 21 11.5z" />,
  dashboard: (
    <>
      <line x1="4" y1="20" x2="4" y2="12" />
      <line x1="12" y1="20" x2="12" y2="6" />
      <line x1="20" y1="20" x2="20" y2="15" />
    </>
  ),
  custom: (
    <>
      <line x1="4" y1="6" x2="20" y2="6" />
      <circle cx="9" cy="6" r="2" />
      <line x1="4" y1="12" x2="20" y2="12" />
      <circle cx="15" cy="12" r="2" />
      <line x1="4" y1="18" x2="20" y2="18" />
      <circle cx="9" cy="18" r="2" />
    </>
  ),
};

export function CategoryIcon({ category, className = 'w-4 h-4' }: CategoryIconProps) {
  return (
    <svg {...iconProps} className={className} aria-hidden="true">
      {icons[category] || icons.custom}
    </svg>
  );
}
