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

## 8. Settings workspace

### Brief, personas, and taste constraints

- **Brief**: let a macOS user adjust appearance, persistent `oct` configuration, and terminal-only utility actions without having to scan one long form.
- **Primary persona**: a developer making a quick, low-risk preference change between work sessions; the selected tab and save state must be immediately clear.
- **Accessibility persona**: a keyboard and VoiceOver user; native tab controls, real text labels, visible disabled states, and inline outcome messages are required.
- **Taste constraint**: retain the quiet, factual macOS utility character. Use native controls and tonal system surfaces; no decorative gradients, custom hover effects, or non-semantic motion.

### Layout

- Settings content width: 640px.
- Settings content minimum height: 480px.
- Tab content scrolls vertically when needed; the tab selector remains visible.
- Tab content uses `space-4` outer padding, `space-3` section spacing, and `space-2` control-row spacing.

### Primitives

#### Settings tabs
- **Structure**: three native SwiftUI button tabs: General, Configuration, and Tools.
- **Variants**: selected and unselected native system states.
- **States**: the active tab always exposes its task-specific content; switching tabs never discards an in-progress configuration draft.
- **Accessibility**: each tab has a text label and SF Symbol, works with native keyboard focus, and announces selection through SwiftUI.
- **Motion**: native tab transition only.

#### Settings section card
- **Structure**: section icon, title, optional supporting copy, and grouped controls on `macos-control-background`.
- **Spacing**: `space-3` inset, `space-2` control rhythm, 10px continuous corner radius.
- **States**: default, loading, disabled, validation warning, and success/error message.
- **Accessibility**: all controls retain native SwiftUI semantics; supporting copy and messages remain selectable text rather than image-rendered content.

#### Settings action tile
- **Structure**: an SF Symbol, title, and plain-language description in a two-column grid.
- **States**: default, keyboard focus, pressed, and inline launch result. Disabled only when an action cannot be initiated.
- **Accessibility**: the full visible label is the button label; icon is supplemental.

### Interaction and feedback

- Loading, saving, validation, success, and failure states appear inline in the Configuration tab without moving the active controls.
- Save is unavailable until at least one provider is enabled; the warning explains the constraint where it occurs.
- Revert restores the last loaded configuration without leaving the Configuration tab.
- Terminal actions state that they open Terminal before launch and report the launch result inline.

### Accepted design debt and handoff

- Configuration load and save currently call the CLI synchronously. The existing loading/saving state prevents ambiguous interaction, but a future service-layer async conversion should prevent a slow local executable from briefly blocking the Settings window.
- Validate keyboard tab traversal, VoiceOver labels, and light/dark rendering after each Settings change. The owner is the macOS menubar surface.
