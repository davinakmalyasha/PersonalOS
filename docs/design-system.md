# Design System — Monochrome

## Principle

No color. Hierarchy through **contrast, weight, spacing, borders, and elevation** — not hue. Greys do the work; content provides the color (charts in monochrome gradients, photos/covers where present). Looks editorial, not toy.

## Tokens (HSL, light + dark)

We use CSS variables so Tailwind + shadcn read the same source. App supports light/dark via `class="dark"` on `<html>` (next-themes style) with `prefers-color-scheme` fallback.

```css
/* light */
:root {
  --background: 0 0% 100%;
  --foreground: 0 0% 9%;
  --card: 0 0% 100%;
  --card-foreground: 0 0% 9%;
  --popover: 0 0% 100%;
  --popover-foreground: 0 0% 9%;
  --primary: 0 0% 9%;            /* near-black button */
  --primary-foreground: 0 0% 98%;
  --secondary: 0 0% 96%;
  --secondary-foreground: 0 0% 9%;
  --muted: 0 0% 96%;
  --muted-foreground: 0 0% 45%;
  --accent: 0 0% 96%;
  --accent-foreground: 0 0% 9%;
  --destructive: 0 84% 60%;       /* only red allowed, use sparingly (budget-over, delete) */
  --destructive-foreground: 0 0% 98%;
  --border: 0 0% 90%;
  --input: 0 0% 90%;
  --ring: 0 0% 9%;
  --radius: 0.5rem;
  --chart-1: 0 0% 18%;
  --chart-2: 0 0% 35%;
  --chart-3: 0 0% 55%;
  --chart-4: 0 0% 75%;
  --chart-5: 0 0% 88%;
}
/* dark */
.dark {
  --background: 0 0% 7%;
  --foreground: 0 0% 98%;
  --card: 0 0% 9%;
  --card-foreground: 0 0% 98%;
  --popover: 0 0% 9%;
  --popover-foreground: 0 0% 98%;
  --primary: 0 0% 98%;
  --primary-foreground: 0 0% 9%;
  --secondary: 0 0% 14%;
  --secondary-foreground: 0 0% 98%;
  --muted: 0 0% 14%;
  --muted-foreground: 0 0% 65%;
  --accent: 0 0% 14%;
  --accent-foreground: 0 0% 98%;
  --destructive: 0 62% 30%;
  --destructive-foreground: 0 0% 98%;
  --border: 0 0% 16%;
  --input: 0 0% 16%;
  --ring: 0 0% 83%;
  --chart-1: 0 0% 92%;
  --chart-2: 0 0% 75%;
  --chart-3: 0 0% 55%;
  --chart-4: 0 0% 35%;
  --chart-5: 0 0% 20%;
}
```

Tailwind reads these via `tailwind.config.ts` `colors: { border: "hsl(var(--border))", ... }` (shadcn default).

## Typography

- **Display/heading:** `font-sans` tight tracking (`-0.02em` for h1/h2), weight 600–700.
- **Body:** `font-sans` 14–15px, `leading-6`, `text-foreground` with `text-muted-foreground` for secondary.
- **Mono:** `font-mono` 12–13px for hashes, IDs, amounts (tabular-nums on amounts).
- No more than 2 font families; system stack preferred (avoid loading 3 webfonts).

## Elevation (no color shadows)

- `card` = `border` + `bg-card`.
- `hover` = `bg-accent` or `border` intensify (`border-foreground/10 → border-foreground/20`).
- `focus` = `ring` (1px, high contrast).
- Shadows are allowed but minimal (`shadow-sm` only, never colored).

## Components (shadcn mapping)

| shadcn | Monochrome treatment |
|---|---|
| Button | `default` = near-black/near-white (primary), `secondary` muted, `ghost` text-only, `destructive` only for delete/budget-over |
| Card | border-only, no shadow by default |
| Badge | `secondary` grey, `outline` for tags; no colored variants except one semantic red |
| Table | `border` rows, `muted` header bg, tabular-nums for numbers |
| Tabs | underline indicator, not pill, to stay editorial |
| Dialog/Sheet | `border` + `bg-popover` |
| Input/Select | `border-input`, `ring` focus |

## Charts (Recharts)

- Palette = `chart-1…5` (monochrome ramp). Use area/bar/donut with this ramp; tooltip on `bg-popover` + `border`.
- Never use Recharts default colors. Disable legend color dots' hue; rely on pattern/labels.
- Budget bars: track `bg-muted` + fill greys; over-budget segment uses `destructive` muted red.

## Layout

- Sidebar (desktop) = fixed 240px, `border-r`; mobile = Sheet drawer.
- Page max width `max-w-6xl` centered, sections use `space-y-6` and `grid` for cards.
- Global search bar in top nav (filters + `k` shortcut — stretch).
- Empty states are explicit ("No transactions yet — import a CSV") with a primary action, not blank tables.

## Dark/light mechanics

- `next-themes` `ThemeProvider` with `attribute="class"` + `defaultTheme="system"` + `enableSystem`.
- Toggle: icon button in header (`Sun`/`Moon` lucide), persisted to `localStorage`.
- Every CSS variable above has both themes; never hardcode `#fff`/`#000` in components (use `bg-background`, `text-foreground`).

## Accessibility

- Contrast: `foreground` on `background` ≥ 4.5:1 (both themes). Checked against the tokens above.
- Focus ring always visible; interactive targets ≥ 36px touch size.
- Charts have a `<caption>`/summary text alternative.

## Don't

- No gradient accents, no colored illustrations for "empty state delight".
- No per-pillar accent color — the whole app is one editorial system; pillars differentiate by icon + layout, not hue.
