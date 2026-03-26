# OCI Image Explorer -- UI Style Guide

## Personality: "The Inspector"

OCI Image Explorer is a **precision instrument** for container image anatomy. It dissects images -- fast. Think digital calipers, not a dashboard.

**Voice:** Direct, technical, terse. Speak the developer's language. No marketing copy, no hand-holding, no filler.

**Temperament:** Confident and warm. The orange accent says "active, inspect, discover." The tool doesn't try to impress with decoration. It presents structured data clearly and gets out of the way.

**The test:** If a developer running `crane manifest` could get the answer faster, the UI has failed.

## Design Principles

### 1. Information first, decoration never

Every pixel earns its place by communicating something. Color, weight, and size create hierarchy -- not visual interest. If an element is decorative, remove it.

### 2. Scannable before readable

Developers scan, then drill down. Stats visible without scrolling. Section headers with counts. Collapsible sections for progressive disclosure.

### 3. One accent, everything else neutral

**Orange** (`orange-500` / `#f97316`) is the single interactive accent. It means "you can act on this" -- the Inspect button, the Scan button, active tabs, focus states.

Exception: Semantic colors for referrer types and vulnerability severity. These are functional color-coding, not decoration.

### 4. Density over whitespace

Developer tools should be dense. Tight spacing (`gap-2`, `gap-3`). Whitespace separates groups, not individual items.

### 5. Sharp, not soft

`rounded-md` maximum. No `rounded-lg`, `rounded-xl`, or `rounded-2xl`. Sharper corners feel more technical and intentional.

## Color System

### Core Palette

| Role | Token | Usage |
| --- | --- | --- |
| Background | `slate-900` | Page background |
| Surface | `slate-800` | Cards, headers |
| Surface alt | `slate-800/50` | Nested containers |
| Stat chip bg | `slate-700/40` | Inline stat chips |
| Border | `slate-700` | Card borders, dividers |
| Text primary | `slate-100` | Headings, values |
| Text secondary | `slate-300` | Labels, secondary values |
| Text muted | `slate-400` | Descriptions, icons |
| Text faint | `slate-500` | Tertiary info |
| **Accent** | `orange-500` | Primary buttons, active tabs, focus rings |
| Accent hover | `orange-400` | Button hover |
| Accent bg | `orange-500/15` | Active tab background |
| Accent text | `orange-300` | Active tab text |
| Disabled | `slate-700` bg, `slate-500` text | Disabled buttons |

### Semantic Colors (Functional Only)

| Context | Colors | Usage |
| --- | --- | --- |
| Signatures | `amber-500/20`, `amber-300` | Cosign/Notation |
| SBOMs | `cyan-500/20`, `cyan-300` | Software Bill of Materials |
| Attestations | `violet-500/20`, `violet-300` | SLSA/in-toto |
| VEX | `emerald-500/20`, `emerald-300` | Vulnerability Exploitability eXchange |
| Critical severity | `red-500` | Vulnerability severity |
| High severity | `orange-500` | Vulnerability severity |
| Medium severity | `yellow-500` | Vulnerability severity |
| Low severity | `blue-500` | Vulnerability severity |
| Security score | Dynamic (green/yellow/orange/red) | Supply chain score ring |

### Rules

- **No rainbow stats.** All stat numbers use `text-slate-100` except the hero stat (total size) which uses `text-orange-400`.
- **No gradients.** Solid colors only.
- **Gray icons.** Section header icons are `slate-400`.
- **Orange = action.** Never use orange decoratively.

## Typography

| Element | Classes |
| --- | --- |
| Page title | `text-xl font-bold` |
| Section heading | `font-semibold text-slate-100` |
| Repository name | `text-lg font-semibold` |
| Hero stat value | `text-lg font-bold font-mono text-orange-400` |
| Stat value | `text-base font-semibold font-mono text-slate-100` |
| Stat label | `text-xs text-slate-400` |
| Count badge | `text-xs bg-slate-600/50 text-slate-300 rounded` |
| Code/digest | `text-xs font-mono text-slate-300` |

## Component Patterns

### Stats Row

Compact horizontal chips. Hero stat (total size) is slightly larger and uses the accent color. All others are neutral.

### Collapsible Sections

- Card: `border border-slate-700 rounded-md`
- Icon: `w-5 h-5 text-slate-400` (neutral)
- Badge: `bg-slate-600/50 text-slate-300 rounded` (not blue, not rounded-full)
- Chevron rotates on toggle

### Buttons

| Type | Classes |
| --- | --- |
| Primary (Inspect/Scan) | `bg-orange-500 hover:bg-orange-400 rounded-md` |
| Tab active | `bg-orange-500/15 text-orange-300` |
| Tab inactive | `text-slate-400 hover:text-slate-200` |
| Ghost | `bg-slate-800 hover:bg-slate-700 border-slate-600` |
| Disabled | `bg-slate-700 text-slate-500 cursor-not-allowed` |

## Motion

- Fade-in: `0.3s ease-out` for content after API response
- Score ring: `0.8s ease-out` fill animation
- All animations respect `prefers-reduced-motion`
- No bounce, no elastic, no spring

## Accessibility

- `prefers-reduced-motion` disables all animations
- Keyboard: Enter to submit, Tab to navigate, Enter/Space for collapsible toggles
- Semantic HTML throughout
- Color is never the sole differentiator

## Anti-Patterns

Do not introduce:

- Blue-purple gradient anything
- Multiple neon accent colors on stats
- Large decorative icons above headings
- Centered paragraphs explaining what the tool does
- `alert()` for errors
- `rounded-lg` or larger on cards
- Glassmorphism or blur effects
- Generic system font stacks presented as "design"
