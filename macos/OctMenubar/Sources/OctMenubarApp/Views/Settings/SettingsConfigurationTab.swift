import SwiftUI

struct SettingsConfigurationTab: View {
    @AppStorage(MenubarPreferences.useProviderAccentColorsKey) private var useProviderAccentColors = true
    @Binding var configDraft: ConfigurationDraft?

    let isLoading: Bool
    let feedback: SettingsFeedback?
    let onDraftChange: () -> Void
    let onLoad: () -> Void

    var body: some View {
        ScrollView(.vertical) {
            VStack(alignment: .leading, spacing: 12) {
                configurationContent

                if let feedback {
                    SettingsFeedbackMessage(feedback: feedback)
                }
            }
            .padding(.vertical, 12)
            .frame(maxWidth: .infinity, alignment: .leading)
        }
    }

    @ViewBuilder
    private var configurationContent: some View {
        if isLoading && configDraft == nil {
            SettingsSectionCard(
                title: "Configuration",
                systemImage: "arrow.triangle.2.circlepath",
                description: "Reading the current oct configuration."
            ) {
                ProgressView("Loading configuration")
            }
        } else if configDraft == nil {
            SettingsSectionCard(
                title: "Configuration",
                systemImage: "slider.horizontal.3",
                description: "Load the current oct configuration before making changes."
            ) {
                Button(action: onLoad) {
                    Label("Load configuration", systemImage: "arrow.clockwise")
                }
                .buttonStyle(.bordered)
            }
        } else {
            providerSection
            appearanceSection
            displaySection
            alertSection
            sessionRefreshSection
        }
    }

    private var providerSection: some View {
        SettingsSectionCard(
            title: "Providers",
            systemImage: "person.2",
            description: "Choose usage providers and change execution priority with the arrow controls."
        ) {
            VStack(alignment: .leading, spacing: 8) {
                LabeledContent("Configuration file") {
                    Text(configDraft?.configFile ?? "")
                        .font(.system(size: 11))
                        .foregroundStyle(.secondary)
                        .lineLimit(2)
                        .multilineTextAlignment(.trailing)
                        .textSelection(.enabled)
                }

                Divider()

                ForEach(Array((configDraft?.tools ?? []).enumerated()), id: \.element.id) { index, tool in
                    HStack(spacing: 8) {
                        Toggle(tool.name, isOn: toolEnabledBinding(tool.binaryName))

                        Spacer(minLength: 8)

                        Text("#\(index + 1)")
                            .font(.system(size: 11, weight: .medium))
                            .foregroundStyle(.secondary)
                            .frame(width: 28, alignment: .trailing)

                        Button {
                            moveTool(tool.binaryName, by: -1)
                        } label: {
                            Image(systemName: "chevron.up")
                        }
                        .buttonStyle(.borderless)
                        .disabled(index == 0)
                        .accessibilityLabel("Move \(tool.name) up")

                        Button {
                            moveTool(tool.binaryName, by: 1)
                        } label: {
                            Image(systemName: "chevron.down")
                        }
                        .buttonStyle(.borderless)
                        .disabled(index == (configDraft?.tools.count ?? 0) - 1)
                        .accessibilityLabel("Move \(tool.name) down")
                    }
                }

                if configDraft?.hasEnabledTool == false {
                    SettingsFeedbackMessage(
                        feedback: .warning("Select at least one provider."),
                        compact: true
                    )
                }
            }
        }
    }

    private var displaySection: some View {
        SettingsSectionCard(
            title: "Usage display",
            systemImage: "chart.bar",
            description: "Cards and menu bar always show remaining quota."
        ) {
            VStack(alignment: .leading, spacing: 8) {
                Picker("Menu bar display", selection: menubarTitleModeBinding) {
                    ForEach(MenubarTitleMode.allCases) { mode in
                        Text(mode.label).tag(mode)
                    }
                }
                .pickerStyle(.segmented)
            }
        }
    }

    private var appearanceSection: some View {
        SettingsSectionCard(
            title: "Appearance",
            systemImage: "paintpalette",
            description: "Use provider colors for names and metric chips while preserving semantic status colors."
        ) {
            Toggle("Use provider accent colors", isOn: $useProviderAccentColors)
        }
    }

    private var alertSection: some View {
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
                    alertPercentSlider(title: "Alert threshold", binding: alertMainThresholdBinding)
                    alertPercentSlider(title: "5h", binding: alertFiveHoursThresholdBinding, indent: 18, secondaryLabel: true)
                    alertPercentSlider(title: "7d", binding: alertSevenDaysThresholdBinding, indent: 18, secondaryLabel: true)
                    alertPercentSlider(title: "1m", binding: alertMonthlyThresholdBinding, indent: 18, secondaryLabel: true)

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
    /// percent chip so values don't jitter while dragging. The 1...99 range
    /// excludes zero and 100 — both rejected by the CLI payload contract.
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
            ThresholdSlider(value: binding, range: 1...99, color: thresholdTrackColor(binding.wrappedValue))
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

    private var sessionRefreshSection: some View {
        SettingsSectionCard(
            title: "Session refresh",
            systemImage: "arrow.clockwise.circle",
            description: "Keep supported provider sessions current on a regular schedule."
        ) {
            VStack(alignment: .leading, spacing: 8) {
                Toggle("Refresh sessions automatically", isOn: sessionRefreshEnabledBinding)

                Picker("Refresh interval", selection: sessionRefreshIntervalBinding) {
                    ForEach(SessionRefreshIntervalOption.all) { option in
                        Text(option.label).tag(option.value)
                    }
                }

                if sessionRefreshUsesHour {
                    Stepper(value: sessionRefreshHourBinding, in: 0...23) {
                        Text("Refresh hour: \(configDraft?.sessionRefreshHour ?? 0):00")
                    }
                } else {
                    LabeledContent("Refresh hour") {
                        Text("Not used for sub-daily intervals")
                            .foregroundStyle(.secondary)
                    }
                }
            }
        }
    }

    private var menubarTitleModeBinding: Binding<MenubarTitleMode> {
        Binding(
            get: { configDraft?.menubarTitleMode ?? .oct },
            set: {
                configDraft?.setMenubarTitleMode($0)
                onDraftChange()
            }
        )
    }

    private var sessionRefreshEnabledBinding: Binding<Bool> {
        Binding(
            get: { configDraft?.sessionRefreshEnabled ?? false },
            set: {
                configDraft?.sessionRefreshEnabled = $0
                onDraftChange()
            }
        )
    }

    private var sessionRefreshIntervalBinding: Binding<String> {
        Binding(
            get: { SessionRefreshIntervalOption.option(for: configDraft?.sessionRefreshInterval ?? "daily").value },
            set: {
                configDraft?.sessionRefreshInterval = $0
                onDraftChange()
            }
        )
    }

    private var sessionRefreshHourBinding: Binding<Int> {
        Binding(
            get: { configDraft?.sessionRefreshHour ?? 9 },
            set: {
                configDraft?.sessionRefreshHour = $0
                onDraftChange()
            }
        )
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

    /// The main alert threshold is the effective default: reading resolves to
    /// thresholds.default and writing keeps the legacy threshold_percent in
    /// sync, since thresholdFor resolves window -> default -> legacy percent
    /// and a diverging legacy value would resurface via the CLI.
    private var alertMainThresholdBinding: Binding<Double> {
        Binding(
            get: { configDraft?.alert.thresholds.defaultThreshold ?? AlertSettings.goDefaults.thresholds.defaultThreshold },
            set: {
                configDraft?.alert.thresholds.defaultThreshold = $0
                configDraft?.alert.thresholdPercent = $0
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
    private var sessionRefreshUsesHour: Bool {
        SessionRefreshIntervalOption.usesHour(configDraft?.sessionRefreshInterval ?? "daily")
    }


    private func toolEnabledBinding(_ binaryName: String) -> Binding<Bool> {
        Binding(
            get: {
                configDraft?.tools.first(where: { $0.binaryName == binaryName })?.enabled ?? false
            },
            set: { enabled in
                configDraft?.setTool(binaryName, enabled: enabled)
                onDraftChange()
            }
        )
    }
    private func moveTool(_ binaryName: String, by offset: Int) {
        configDraft?.moveTool(binaryName, by: offset)
        onDraftChange()
    }
}

/// A slider that keeps its track colored on every macOS style. The macOS 26+
/// control redesign renders sliders in fixed chrome and ignores both `.tint`
/// and `.accentColor`, and the threshold color IS the information here
/// (green → orange → red ramp), so the track and knob are drawn by hand.
/// Values snap to whole numbers to keep the CLI payload contract (1...99,
/// no fractions); the system Slider stays as the accessibility
/// representation for VoiceOver and keyboard interaction.
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
                        value = min(max(raw.rounded(), range.lowerBound), range.upperBound)
                    }
            )
        }
        .frame(height: 18)
        .accessibilityRepresentation {
            Slider(value: $value, in: range)
        }
    }
}
