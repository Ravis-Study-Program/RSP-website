import type { ColumnDefTemplate } from '@tanstack/react-table';
import type { ReactNode } from 'react';

/**
 * Draws a cell by calling its renderer, rather than mounting it as a
 * component as flexRender does. Columns are usually defined inline, so each
 * render brings new renderer functions; as components those would remount
 * every cell whenever the table's parent re-renders, closing any open row
 * menu. Cell renderers must not use hooks.
 */
export function renderCell<TProps extends object>(
  renderer: ColumnDefTemplate<TProps> | undefined,
  props: TProps,
): ReactNode {
  return typeof renderer === 'function'
    ? (renderer as (props: TProps) => ReactNode)(props)
    : renderer;
}
