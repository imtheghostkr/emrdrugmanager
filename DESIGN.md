# Drug Storage Bridge Design System

## 1. Atmosphere & Identity

A quiet, work-focused clinical utility. The signature is a restrained white-and-gray workspace with teal reserved for primary actions and dense tables optimized for repeated scanning.

## 2. Color

| Role | Token | Value | Usage |
|---|---|---|---|
| Page | `--bg` | `#f5f6f8` | Application background |
| Surface | `--surface` | `#ffffff` | Panels, inputs, tables |
| Subtle surface | `--surface-subtle` | `#f9fafb` | Cards and hover context |
| Text | `--text` | `#171b22` | Primary content |
| Muted text | `--muted` | `#687384` | Labels and status text |
| Border | `--line` | `#dfe3ea` | Default separators |
| Strong border | `--line-strong` | `#c8d0dc` | Hovered controls |
| Primary | `--primary` | `#1f6f68` | Primary commands |
| Primary hover | `--primary-hover` | `#185a55` | Primary command hover |
| Focus | `--focus` | `#8bbf47` | Keyboard focus ring |
| Danger | `--danger` | `#b42318` | Destructive states |
| Warning | `--warn` | `#9a6700` | Cautions |

Colors are defined in `styles.css`; new colors require a semantic token before use.

## 3. Typography

- Primary stack: Pretendard, Noto Sans KR, Malgun Gothic, system UI, sans-serif.
- Body: 14px at 1.45 line height.
- H1: 20px, strong weight. H2: 16px, strong weight.
- Table content: 13px; labels and metadata: 12px.
- Letter spacing remains `0` for Korean readability.

## 4. Spacing & Layout

- Base unit: 4px; existing spacing uses multiples or compact 2px adjustments.
- Main content width: 1360px, centered, with 24px desktop and 14px mobile padding.
- Panels use 20px desktop and 16px mobile padding.
- Repeated control gaps use 10px to 14px; section separation uses 12px to 24px.
- The mobile breakpoint is 720px; action rows stack vertically below it.

## 5. Components

### Form Controls
- Structure: native input, select, textarea, and button elements.
- States: default, hover, focus-visible, active for buttons, disabled through native semantics.
- Accessibility: visible or accessible names, 38px stable height, high-contrast focus outline.

### Action Row
- Structure: `.actions` for commands and `.inline` for an expanding input with adjacent controls.
- States: horizontal on desktop and stacked on mobile.
- Spacing: 10px gap with 12px vertical margin.

### Data Table
- Structure: semantic table rendered into a bounded scrolling container.
- States: loading status before render, empty message, sticky header, row hover.
- Accessibility: real headings and cells; content wraps without clipping.

### Tabs
- Structure: native buttons with `data-tab` targets.
- States: default, hover, active, and focus-visible.

## 6. Motion & Interaction

- Micro-interactions use 80ms to 120ms transitions for button press, color, border, and focus feedback.
- Motion is limited to interaction feedback; there are no decorative animations.
- Filtering updates immediately from user input without moving surrounding controls.

## 7. Depth & Surface

The strategy is mixed but restrained: one-pixel borders define controls and tables, while panels use the shared subtle two-layer `--shadow`. Nested decorative cards are avoided; cards are limited to compact summary metrics.
