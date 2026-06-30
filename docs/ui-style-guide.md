# XFile UI Style Guide

This guide defines the default visual language for the XFile admin UI. The baseline direction is **iOS 26 Clean Glass**: calm, light, translucent, readable, and suitable for a file-management console.

The guide is intentionally token-driven. Values can be adjusted later without changing the product direction.

## Design Direction

- Style name: iOS 26 Clean Glass
- Keywords: light glass, soft depth, clear hierarchy, compact admin layout, high readability
- Main goal: make file management, sharing, logs, and settings feel modern without reducing scanning speed
- Avoid: heavy gradients, low-contrast glass, oversized marketing layouts, deeply nested cards, one-color themes

## Element Plus Strategy

Element Plus is already used across most pages, so the recommended approach is gradual:

- Keep Element Plus for tables, forms, dialogs, pagination, date picker, select, switch, loading, message, and message box.
- Override Element Plus through theme variables, CSS variables, and local wrapper classes.
- Do not rely on raw Element Plus defaults for spacing, radius, button width, or colors.
- Create project-level components for high-frequency UI when needed: `XButton`, `XIconButton`, `XPanel`, `XToolbar`, `XTableActions`, `XEmptyState`.
- Replace Element Plus gradually only where styling becomes painful or UX needs are custom.

Recommended rule: do not remove Element Plus in one pass. First unify tokens, then wrap common controls, then replace page by page.

## Color Tokens

### Light Mode

| Token | Value | Usage |
| --- | --- | --- |
| `--x-color-bg` | `#F5F7FA` | App background |
| `--x-color-bg-soft` | `#F8FAFC` | Secondary page bands |
| `--x-color-surface` | `rgba(255, 255, 255, 0.72)` | Glass panels |
| `--x-color-surface-solid` | `#FFFFFF` | Dense tables and forms |
| `--x-color-surface-raised` | `rgba(255, 255, 255, 0.84)` | Header, sidebar, popovers |
| `--x-color-border` | `rgba(17, 24, 39, 0.10)` | Default border |
| `--x-color-border-strong` | `rgba(17, 24, 39, 0.16)` | Focused or emphasized border |
| `--x-color-text` | `#111827` | Primary text |
| `--x-color-text-muted` | `#6B7280` | Secondary text |
| `--x-color-text-subtle` | `#9CA3AF` | Placeholder, meta text |
| `--x-color-primary` | `#007AFF` | Primary actions and active nav |
| `--x-color-primary-hover` | `#006EE6` | Primary hover |
| `--x-color-primary-soft` | `#EAF3FF` | Primary soft background |
| `--x-color-success` | `#34C759` | Success |
| `--x-color-warning` | `#FF9500` | Warning |
| `--x-color-danger` | `#FF3B30` | Dangerous actions |
| `--x-color-info` | `#5AC8FA` | Informational states |

### Dark Mode

Dark mode should keep the same structure, but reduce transparency and increase contrast.

| Token | Value | Usage |
| --- | --- | --- |
| `--x-color-bg` | `#0B0F14` | App background |
| `--x-color-bg-soft` | `#111827` | Secondary page bands |
| `--x-color-surface` | `rgba(24, 31, 42, 0.78)` | Glass panels |
| `--x-color-surface-solid` | `#151B24` | Dense tables and forms |
| `--x-color-surface-raised` | `rgba(28, 36, 48, 0.86)` | Header, sidebar, popovers |
| `--x-color-border` | `rgba(255, 255, 255, 0.10)` | Default border |
| `--x-color-border-strong` | `rgba(255, 255, 255, 0.18)` | Focused or emphasized border |
| `--x-color-text` | `#F9FAFB` | Primary text |
| `--x-color-text-muted` | `#CBD5E1` | Secondary text |
| `--x-color-text-subtle` | `#94A3B8` | Placeholder, meta text |
| `--x-color-primary` | `#0A84FF` | Primary actions and active nav |
| `--x-color-primary-hover` | `#409CFF` | Primary hover |
| `--x-color-primary-soft` | `rgba(10, 132, 255, 0.16)` | Primary soft background |
| `--x-color-success` | `#30D158` | Success |
| `--x-color-warning` | `#FFD60A` | Warning |
| `--x-color-danger` | `#FF453A` | Dangerous actions |
| `--x-color-info` | `#64D2FF` | Informational states |

## Radius Tokens

| Token | Value | Usage |
| --- | --- | --- |
| `--x-radius-page` | `16px` | Large page panels or hero bands |
| `--x-radius-panel` | `14px` | Cards, panels, dialogs |
| `--x-radius-control` | `10px` | Buttons, inputs, selects |
| `--x-radius-small` | `8px` | Tags, compact buttons, small icons |
| `--x-radius-round` | `999px` | Pills, avatar-like controls |

Do not use random radii per page. If a component needs a different radius, add a token first.

## Typography

Use the existing system font stack:

```css
Inter, system-ui, Avenir, "Helvetica Neue", Helvetica, "PingFang SC",
"Hiragino Sans GB", "Microsoft YaHei", Arial, sans-serif
```

| Token | Value | Usage |
| --- | --- | --- |
| `--x-font-size-xs` | `12px` | Meta text, hints, compact tags |
| `--x-font-size-sm` | `13px` | Table meta, descriptions |
| `--x-font-size-md` | `14px` | Default UI text |
| `--x-font-size-lg` | `16px` | Panel titles |
| `--x-font-size-xl` | `20px` | Page section title |
| `--x-font-size-page` | `28px` | Page title |

Line heights:

- Compact controls: `1.2`
- Body/UI text: `1.5`
- Descriptions: `1.6`

Font weights:

- Normal text: `400`
- Important labels: `600`
- Page and panel titles: `700`

Letter spacing should stay `0`.

## Spacing

| Token | Value | Usage |
| --- | --- | --- |
| `--x-space-1` | `4px` | Tiny gaps |
| `--x-space-2` | `8px` | Icon and text gap |
| `--x-space-3` | `12px` | Compact groups |
| `--x-space-4` | `16px` | Default panel padding |
| `--x-space-5` | `20px` | Page group gap |
| `--x-space-6` | `24px` | Main page padding |
| `--x-space-7` | `32px` | Large page sections |

Admin pages should feel compact. Prefer `16px` to `24px` panel padding unless the page is public-facing.

## Buttons

### Sizes

| Size | Height | Min Width | Padding | Radius | Usage |
| --- | --- | --- | --- | --- | --- |
| Small | `32px` | `32px` icon-only, `72px` text | `0 12px` | `10px` | Table actions, toolbar secondary actions |
| Default | `36px` | `36px` icon-only, `88px` text | `0 14px` | `10px` | Most actions |
| Large | `44px` | `44px` icon-only, `112px` text | `0 18px` | `12px` | Login, public share primary actions |

### Variants

- Primary: blue fill, white text.
- Secondary: translucent white surface, primary text or default text.
- Text: no fill, used only in dense table/action areas.
- Danger: red text or red fill only for destructive confirmation actions.
- Icon-only: square, use tooltips or `title`.

### Button Rules

- Use icons for common actions: upload, new folder, download, share, link, rename, delete, refresh, search, copy.
- Keep button labels short: two to four Chinese characters where possible.
- Do not mix multiple button heights in the same toolbar.
- Destructive buttons should be visually quieter until confirmation is needed.

## Panels And Glass

Default panel:

```css
background: var(--x-color-surface);
border: 1px solid var(--x-color-border);
border-radius: var(--x-radius-panel);
backdrop-filter: blur(18px) saturate(1.15);
box-shadow: var(--x-shadow-panel);
```

Use solid surfaces for:

- Tables with many rows
- Long forms
- Log pages
- Dense settings pages

Use glass surfaces for:

- Header
- Sidebar
- Dashboard summary panels
- Public share landing states
- Small summary cards

## Shadows

| Token | Value | Usage |
| --- | --- | --- |
| `--x-shadow-subtle` | `0 1px 2px rgba(15, 23, 42, 0.06)` | Controls and light surfaces |
| `--x-shadow-panel` | `0 12px 32px rgba(15, 23, 42, 0.08)` | Panels |
| `--x-shadow-floating` | `0 18px 48px rgba(15, 23, 42, 0.14)` | Dialogs, popovers |

Dark mode shadows should be weaker and rely more on borders.

## Tables

- Header height: `44px`
- Row height: `48px` minimum
- Table background: solid surface, not heavy glass
- Header text: `13px`, weight `600`, muted color
- Body text: `14px`
- Row hover: primary soft background
- File/folder name column should be strongest visual priority
- Action column should use icon-only or very short text buttons

## Forms

- Input height: `36px`
- Large login/share input height: `44px`
- Label color: primary text
- Help text color: muted text
- Placeholder color: subtle text
- Input radius: `10px`
- Input background: solid or lightly translucent surface
- Focus border: primary color
- Form sections should use `16px` to `20px` vertical spacing

## Navigation

Sidebar:

- Width: `220px` to `248px`
- Item height: `40px`
- Item radius: `10px`
- Active item background: primary soft
- Active item text/icon: primary
- Inactive item text: muted

Header:

- Height: `64px`
- Background: raised glass
- Border bottom: default border
- Search entry height: `40px`
- Header actions: icon-only buttons, default `36px`

## Tags And Status

| Status | Text | Background |
| --- | --- | --- |
| Success | `--x-color-success` | `rgba(52, 199, 89, 0.12)` |
| Warning | `--x-color-warning` | `rgba(255, 149, 0, 0.14)` |
| Danger | `--x-color-danger` | `rgba(255, 59, 48, 0.12)` |
| Info | `--x-color-info` | `rgba(90, 200, 250, 0.14)` |
| Neutral | `--x-color-text-muted` | `rgba(107, 114, 128, 0.12)` |

Tags should use `8px` radius and `24px` height by default.

## Dialogs And Popovers

- Dialog radius: `16px`
- Dialog width: use content-driven widths, generally `420px`, `560px`, `720px`, or `960px`
- Overlay: `rgba(15, 23, 42, 0.34)` in light mode, `rgba(0, 0, 0, 0.52)` in dark mode
- Dialog background: raised glass for small dialogs, solid surface for complex forms
- Footer buttons: right aligned, consistent height

## Page Patterns

Dashboard:

- Use a compact overview band plus metric cards.
- Avoid decorative-only hero visuals.
- Metrics use icon, label, number, and optional trend/meta.

File Manager:

- Toolbar should remain dense and predictable.
- File table should prioritize name, size, modified time, and actions.
- Preview dialog can use a larger solid surface.

Shares And Direct Links:

- Tables should be dense.
- Link copy/delete actions should be icon-first.
- Password/expiration status should use tags.

Logs:

- Solid surfaces are preferred.
- Filters should wrap cleanly on mobile.
- Pagination stays below the table, right aligned on desktop.

Settings:

- Use simple form sections.
- Avoid cards inside cards.
- Switches should align with labels and supporting text.

Public Share Page:

- Can be more spacious than admin pages.
- Use glass panels, larger icon, and clearer primary action.
- Keep file browsing consistent with admin table behavior.

## Responsive Rules

- Desktop page padding: `24px`
- Tablet page padding: `18px`
- Mobile page padding: `14px`
- Hide sidebar under `1100px`; header keeps brand and actions.
- Tables may scroll horizontally rather than compressing every column.
- Toolbars stack vertically under `760px`.
- Text must not overflow buttons, cards, or table cells.

## CSS Variable Starting Point

```css
:root {
  --x-color-bg: #F5F7FA;
  --x-color-bg-soft: #F8FAFC;
  --x-color-surface: rgba(255, 255, 255, 0.72);
  --x-color-surface-solid: #FFFFFF;
  --x-color-surface-raised: rgba(255, 255, 255, 0.84);
  --x-color-border: rgba(17, 24, 39, 0.10);
  --x-color-border-strong: rgba(17, 24, 39, 0.16);
  --x-color-text: #111827;
  --x-color-text-muted: #6B7280;
  --x-color-text-subtle: #9CA3AF;
  --x-color-primary: #007AFF;
  --x-color-primary-hover: #006EE6;
  --x-color-primary-soft: #EAF3FF;
  --x-color-success: #34C759;
  --x-color-warning: #FF9500;
  --x-color-danger: #FF3B30;
  --x-color-info: #5AC8FA;

  --x-radius-page: 16px;
  --x-radius-panel: 14px;
  --x-radius-control: 10px;
  --x-radius-small: 8px;
  --x-radius-round: 999px;

  --x-space-1: 4px;
  --x-space-2: 8px;
  --x-space-3: 12px;
  --x-space-4: 16px;
  --x-space-5: 20px;
  --x-space-6: 24px;
  --x-space-7: 32px;

  --x-shadow-subtle: 0 1px 2px rgba(15, 23, 42, 0.06);
  --x-shadow-panel: 0 12px 32px rgba(15, 23, 42, 0.08);
  --x-shadow-floating: 0 18px 48px rgba(15, 23, 42, 0.14);
}

.dark {
  --x-color-bg: #0B0F14;
  --x-color-bg-soft: #111827;
  --x-color-surface: rgba(24, 31, 42, 0.78);
  --x-color-surface-solid: #151B24;
  --x-color-surface-raised: rgba(28, 36, 48, 0.86);
  --x-color-border: rgba(255, 255, 255, 0.10);
  --x-color-border-strong: rgba(255, 255, 255, 0.18);
  --x-color-text: #F9FAFB;
  --x-color-text-muted: #CBD5E1;
  --x-color-text-subtle: #94A3B8;
  --x-color-primary: #0A84FF;
  --x-color-primary-hover: #409CFF;
  --x-color-primary-soft: rgba(10, 132, 255, 0.16);
  --x-color-success: #30D158;
  --x-color-warning: #FFD60A;
  --x-color-danger: #FF453A;
  --x-color-info: #64D2FF;
}
```

## Element Plus Mapping

Map project tokens to Element Plus variables instead of using Element Plus defaults directly.

```css
:root {
  --ep-color-primary: var(--x-color-primary);
  --ep-color-success: var(--x-color-success);
  --ep-color-warning: var(--x-color-warning);
  --ep-color-danger: var(--x-color-danger);
  --ep-color-error: var(--x-color-danger);
  --ep-color-info: var(--x-color-info);
  --ep-text-color-primary: var(--x-color-text);
  --ep-text-color-regular: var(--x-color-text);
  --ep-text-color-secondary: var(--x-color-text-muted);
  --ep-text-color-placeholder: var(--x-color-text-subtle);
  --ep-border-color: var(--x-color-border);
  --ep-border-radius-base: var(--x-radius-control);
  --ep-fill-color-blank: var(--x-color-surface-solid);
}
```

## Migration Checklist

1. Add the token variables to `web/src/styles/index.scss`.
2. Update Element Plus SCSS theme values to use the blue iOS 26 palette.
3. Replace hard-coded project colors with `--x-*` tokens.
4. Normalize panel, toolbar, table, button, input, and dialog radii.
5. Add dark mode token overrides before visual tuning.
6. Create small wrapper components only after the repeated patterns are clear.
7. Verify pages in both light and dark modes after each batch.

