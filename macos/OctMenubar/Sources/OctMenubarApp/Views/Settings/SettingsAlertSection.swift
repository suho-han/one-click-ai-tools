import SwiftUI

/// Alerts section of the settings tab: enable toggle, shared/per-window
/// thresholds, cooldown, and the quiet timer. Extracted from
/// SettingsConfigurationTab so the tab file stays navigable; state lives in
/// the shared draft and the shared-threshold preference keeps its
/// AppStorage key.
struct SettingsAlertSection: View {
    @Binding var configDraft: ConfigurationDraft?
    let onDraftChange: () -> Void
    @AppStorage(MenubarPreferences.sharedAlertThresholdKey) private var sharesAlertThreshold = false

    var body: some View {
        SettingsSectionCard(
            title: "Alerts",
            systemImage: "bell.badge",
            description: "Set global usage notification preferences."
        ) {
            VStack(alignment: .leading, spacing: 12) {
                Toggle("Enable usage alerts", isOn: alertEnabledBinding)

                // The controls only matter while alerts are on, but `.disabled`
                // would gray out the sliders and erase their threshold colors
                // (macOS renders disabled controls in gray regardless of tint),
                // so gate interaction instead and dim with opacity alone.
                VStack(alignment: .leading, spacing: 10) {
                    Toggle("Use the same threshold for 5h, 7d, and 1m", isOn: sharedThresholdBinding)

                    if sharesAlertThreshold {
                        alertPercentSlider(title: "Alert threshold", binding: alertMainThresholdBinding)
                    } else {
                        alertPercentSlider(title: "5h", binding: alertFiveHoursThresholdBinding, indent: 18, secondaryLabel: true)
                        alertPercentSlider(title: "7d", binding: alertSevenDaysThresholdBinding, indent: 18, secondaryLabel: true)
                        alertPercentSlider(title: "1m", binding: alertMonthlyThresholdBinding, indent: 18, secondaryLabel: true)
                    }

                    alertPickerRow(title: "Cooldown", selection: alertCooldownMinutesBinding) {
                        ForEach(AlertSettings.cooldownMenuChoices(current: configDraft?.alert.cooldownMinutes ?? 360), id: \.self) { minutes in
                            Text(AlertSettings.cooldownLabel(for: minutes)).tag(minutes)
                        }
                    }

                    alertPickerRow(title: "Quiet timer", selection: alertQuietChoiceBinding) {
                        Text("Off").tag(0)
                        ForEach(AlertSettings.quietChoices, id: \.self) { hours in
                            Text("\(hours) hr").tag(hours)
                        }
                    }

                    if let quietUntilDate = alertQuietUntilDate, quietUntilDate > Date() {
                        Text("Quiet until \(quietUntilDate.formatted(date: .omitted, time: .shortened))")
                            .font(.system(size: 11))
                            .foregroundStyle(.secondary)
                    }
                }
                .allowsHitTesting(alertsEnabled)
                .opacity(alertsEnabled ? 1 : 0.45)
            }
        }
    }

    private var alertsEnabled: Bool {
        configDraft?.alert.enabled ?? false
    }

    /// Aligned threshold row: label column, draggable slider, fixed-width
    /// percent chip so values don't jitter while dragging. New selections use
    /// 5% stops through 95%, plus the CLI's 99% ceiling.
    private func alertPercentSlider(
        title: String,
        binding: Binding<Double>,
        indent: CGFloat = 0,
        secondaryLabel: Bool = false
    ) -> some View {
        HStack(spacing: 10) {
            Text(title)
                .font(.system(size: secondaryLabel ? 11 : 12, weight: secondaryLabel ? .regular : .medium))
                .foregroundStyle(secondaryLabel ? Color.secondary : Color.primary)
                .frame(width: 120, alignment: .leading)
                .padding(.leading, indent)
            ThresholdSlider(value: binding, range: 5...99, color: thresholdTrackColor(binding.wrappedValue))
                .accessibilityLabel(title)
            Text("\(Int(binding.wrappedValue.rounded()))%")
                .font(.system(size: 11, weight: .medium).monospacedDigit())
                .foregroundStyle(.secondary)
                .frame(width: 38, alignment: .trailing)
        }
    }

    /// Track color for a threshold slider: green under 50%, orange up to 75%,
    /// then a gradual orange→red ramp toward the 99% ceiling.
    private func thresholdTrackColor(_ percent: Double) -> Color {
        switch percent {
        case ..<50:
            return Color(nsColor: .systemGreen)
        case ..<75:
            return Color(nsColor: .systemOrange)
        default:
            let fraction = min(max((percent - 75) / 24, 0), 1)
            let from = NSColor.systemOrange.usingColorSpace(.sRGB) ?? NSColor.systemOrange
            let to = NSColor.systemRed.usingColorSpace(.sRGB) ?? NSColor.systemRed
            let f = CGFloat(fraction)
            return Color(
                red: Double(from.redComponent + (to.redComponent - from.redComponent) * f),
                green: Double(from.greenComponent + (to.greenComponent - from.greenComponent) * f),
                blue: Double(from.blueComponent + (to.blueComponent - from.blueComponent) * f)
            )
        }
    }

    private func alertPickerRow<Options: View>(
        title: String,
        selection: Binding<Int>,
        @ViewBuilder options: () -> Options
    ) -> some View {
        HStack(spacing: 10) {
            Text(title)
                .font(.system(size: 12))
                .frame(width: 120, alignment: .leading)
            Picker(title, selection: selection, content: options)
                .labelsHidden()
                .frame(width: 180, alignment: .leading)
            Spacer(minLength: 0)
        }
    }

    private var alertEnabledBinding: Binding<Bool> {
        Binding(
            get: { configDraft?.alert.enabled ?? false },
            set: {
                configDraft?.alert.enabled = $0
                onDraftChange()
            }
        )
    }

    /// External CLI edits can break a previously shared configuration; show
    /// the separate controls rather than hiding those divergent values.
    private func reconcileSharedThresholdPreference() {
        guard sharesAlertThreshold, let thresholds = configDraft?.alert.thresholds else { return }
        if !thresholds.hasSharedWindows {
            sharesAlertThreshold = false
        }
    }

    /// Checking adopts the current default for every window. Unchecking
    /// leaves the values intact and individually editable.
    private var sharedThresholdBinding: Binding<Bool> {
        Binding(
            get: { sharesAlertThreshold },
            set: { enabled in
                sharesAlertThreshold = enabled
                if enabled, let percent = configDraft?.alert.thresholds.defaultThreshold {
                    configDraft?.alert.setSharedThreshold(percent)
                    onDraftChange()
                }
            }
        )
    }

    /// A shared edit updates the legacy fallback alongside every window key,
    /// since the CLI checks window -> default -> threshold_percent.
    private var alertMainThresholdBinding: Binding<Double> {
        Binding(
            get: { configDraft?.alert.thresholds.defaultThreshold ?? AlertSettings.goDefaults.thresholds.defaultThreshold },
            set: {
                configDraft?.alert.setSharedThreshold($0)
                onDraftChange()
            }
        )
    }

    private var alertCooldownMinutesBinding: Binding<Int> {
        Binding(
            get: { configDraft?.alert.cooldownMinutes ?? 360 },
            set: {
                configDraft?.alert.cooldownMinutes = $0
                onDraftChange()
            }
        )
    }

    private var alertQuietChoiceBinding: Binding<Int> {
        Binding(
            get: { AlertSettings.quietChoiceHours(for: configDraft?.alert.quietUntil ?? "") },
            set: { hours in
                configDraft?.alert.quietUntil = AlertSettings.quietUntilString(armingHours: hours)
                onDraftChange()
            }
        )
    }

    private var alertQuietUntilDate: Date? {
        AlertSettings.parseQuietUntil(configDraft?.alert.quietUntil ?? "")
    }

    private var alertFiveHoursThresholdBinding: Binding<Double> {
        alertThresholdBinding(\.fiveHours)
    }

    private var alertSevenDaysThresholdBinding: Binding<Double> {
        alertThresholdBinding(\.sevenDays)
    }

    private var alertMonthlyThresholdBinding: Binding<Double> {
        alertThresholdBinding(\.monthly)
    }

    private func alertThresholdBinding(_ keyPath: WritableKeyPath<AlertThresholds, Double>) -> Binding<Double> {
        Binding(
            get: { configDraft?.alert.thresholds[keyPath: keyPath] ?? AlertSettings.goDefaults.thresholds[keyPath: keyPath] },
            set: {
                configDraft?.alert.thresholds[keyPath: keyPath] = $0
                onDraftChange()
            }
        )
    }
}

/// A slider that keeps its track colored on every macOS style. The macOS 26+
/// control redesign renders sliders in fixed chrome and ignores both `.tint`
/// and `.accentColor`, and the threshold color IS the information here
/// (green → orange → red ramp), so the track and knob are drawn by hand.
/// Drag and accessibility edits share the same 5%-step snapping rule, with
/// 99% as the final stop. Existing off-step values remain visible until edited.
private struct ThresholdSlider: View {
    @Binding var value: Double
    var range: ClosedRange<Double>
    let color: Color

    private var fraction: Double {
        let span = range.upperBound - range.lowerBound
        guard span > 0 else { return 0 }
        return min(max((value - range.lowerBound) / span, 0), 1)
    }

    var body: some View {
        GeometryReader { geometry in
            let width = geometry.size.width
            let knobSize: CGFloat = 18
            ZStack(alignment: .leading) {
                Capsule(style: .continuous)
                    .fill(Color(nsColor: .systemGray))
                    .frame(height: 4)
                Capsule(style: .continuous)
                    .fill(color)
                    .frame(width: max(4, width * fraction), height: 4)
                Circle()
                    .fill(.white)
                    .overlay(
                        Circle()
                            .strokeBorder(Color.black.opacity(0.12), lineWidth: 0.5)
                    )
                    .shadow(color: .black.opacity(0.25), radius: 1.5, y: 0.5)
                    .frame(width: knobSize, height: knobSize)
                    .offset(x: min(max(0, width * fraction - knobSize / 2), width - knobSize))
            }
            .frame(width: width, height: knobSize)
            .contentShape(Rectangle())
            .gesture(
                DragGesture(minimumDistance: 0)
                    .onChanged { gesture in
                        guard width > 0 else { return }
                        let raw = range.lowerBound
                            + Double(gesture.location.x / width) * (range.upperBound - range.lowerBound)
                        value = AlertSettings.sliderThreshold(for: raw)
                    }
            )
        }
        .frame(height: 18)
        .accessibilityRepresentation {
            Slider(
                value: Binding(
                    get: { value },
                    set: { value = AlertSettings.sliderThreshold(for: $0) }
                ),
                in: range
            )
        }
    }
}
