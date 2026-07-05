# one-click-tools Design System

## 1. Atmosphere & Identity

one-click-tools should feel like a quiet macOS utility: compact, native, and factual. The signature is restrained utility depth, where system surfaces and SF typography do the work instead of decorative gradients.

## 2. Color

### Palette

| Role | Token | Light | Dark | Usage |
|------|-------|-------|------|-------|
| Surface/primary | macos-window-background | `NSColor.windowBackgroundColor` | system resolved | Main popover background |
| Surface/secondary | macos-control-background | `NSColor.controlBackgroundColor` | system resolved | Provider cards, buttons, metadata panels |
| Surface/muted | macos-quaternary-label | `NSColor.quaternaryLabelColor` at 15% opacity | system resolved | Count pills and passive chips |
| Text/primary | macos-primary-label | SwiftUI `.primary` | system resolved | Headlines and main values |
| Text/secondary | macos-secondary-label | SwiftUI `.secondary` | system resolved | Captions, supporting copy |
| Border/default | macos-separator | `NSColor.separatorColor` | system resolved | Dividers and subtle card outlines |
| Accent/primary | macos-accent | SwiftUI `Color.accentColor` | system resolved | Actions, refresh state, focus |
| Status/success | status-success | Existing provider status tint | existing | OK badges and state dots |
| Status/warning | status-warning | Existing provider status tint | existing | Warning badges and state dots |
| Status/error | status-error | Existing provider status tint | existing | Error badges and state dots |

### Rules
- Prefer dynamic macOS system colors so light/dark appearance and vibrancy stay native.
- Avoid decorative gradients in utility surfaces.
- Provider accent colors are allowed only for provider identity and status semantics.

## 3. Typography

### Scale

| Level | Size | Weight | Line Height | Tracking | Usage |
|-------|------|--------|-------------|----------|-------|
| Popover title | 19px | semibold | system | 0 | Usage overview title |
| Section title | 13px | semibold | system | 0 | Providers, Actions |
| Body | 12px | regular/medium | system | 0 | Summary and button labels |
| Caption | 11px | regular/medium | system | 0 | Notes, refresh state |
| Micro | 9-10px | semibold/bold | system | 0 | Provider metrics and badges |

### Font Stack
- Primary: San Francisco via SwiftUI `.system`
- Mono: none
- Serif: none

### Rules
- Use tabular/rounded number treatment only for compact numeric metadata.
- Keep labels sentence case.

## 4. Spacing & Layout

### Base Unit

All spacing derives from a base of 4px.

| Token | Value | Usage |
|-------|-------|-------|
| space-1 | 4px | Metric gaps |
| space-2 | 8px | Compact inner gaps |
| space-3 | 12px | Card spacing |
| space-4 | 16px | Popover padding and section spacing |
| space-6 | 24px | Larger section separation |

### Grid
- Popover width: 640px
- Popover max height: 620px
- Provider grid: two columns with 10-12px gutters

### Rules
- Popover content must scroll vertically when content exceeds the max height.
- Never rely on screen-height clipping to reveal actions.

## 5. Components

### Menubar popover
- **Structure**: scrollable root containing header, providers, and actions.
- **Variants**: loading, normal, warning/error provider states.
- **Spacing**: 16px root padding, 12-16px section spacing.
- **States**: refresh button disabled while refreshing; settings and quit remain visible via scroll.
- **Accessibility**: native SwiftUI buttons and system colors.
- **Motion**: native popover animation only.

### Provider card
- **Structure**: status dot, provider name, badge, metric chips, plan strip, optional message.
- **Variants**: provider accent colors on/off.
- **Spacing**: 10px inner padding, 6px vertical rhythm.
- **States**: semantic status badge tint.
- **Accessibility**: text remains real SwiftUI content, not image-rendered.
- **Motion**: none.

## 6. Motion & Interaction

### Timing

| Type | Duration | Easing | Usage |
|------|----------|--------|-------|
| Native | system | system | Popover open/close |

### Rules
- Use native AppKit/SwiftUI interaction behavior.
- Do not add custom animation for utility-only state changes unless it clarifies status.

## 7. Depth & Surface

### Strategy

Use tonal-shift with system colors. The popover background is `windowBackgroundColor`; inner panels use `controlBackgroundColor`; dividers use system separator color. Avoid decorative gradients and heavy shadows inside the popover.
