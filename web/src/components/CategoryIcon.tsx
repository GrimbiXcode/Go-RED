import DOMPurify from 'dompurify';
import { categoryIcon, iconByName } from './icons';

export interface CategoryIconProps {
  category: string;
  className?: string;
}

/** Monochrome icon of a node category (Lucide, currentColor). */
export function CategoryIcon({ category, className = 'w-4 h-4' }: CategoryIconProps) {
  const Icon = categoryIcon(category);
  return <Icon className={className} strokeWidth={2} aria-hidden="true" />;
}

export interface NodeIconProps {
  /** `NodeMetadata.icon`: a Lucide icon name ("timer"); inline `<svg>` markup from third-party nodes is sanitized and shown as is. */
  icon?: string;
  category: string;
  className?: string;
}

/** The icon of a node type: by name, else the raw SVG it ships, else its category's icon. */
export function NodeIcon({ icon, category, className = 'w-4 h-4' }: NodeIconProps) {
  const Named = iconByName(icon);
  if (Named) return <Named className={className} strokeWidth={2} aria-hidden="true" />;
  if (icon && icon.startsWith('<svg')) {
    return <span className={`inline-block shrink-0 [&>svg]:w-full [&>svg]:h-full ${className}`} dangerouslySetInnerHTML={{ __html: DOMPurify.sanitize(icon) }} />;
  }
  return <CategoryIcon category={category} className={className} />;
}
