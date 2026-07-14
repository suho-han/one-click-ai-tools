import SwiftUI

struct SettingsGeneralTab: View {
    @Binding var useProviderAccentColors: Bool

    var body: some View {
        ScrollView(.vertical) {
            VStack(alignment: .leading, spacing: 12) {
                SettingsSectionCard(
                    title: "Provider accents",
                    systemImage: "paintpalette",
                    description: "Keep semantic status colors and add provider colors to names and metric chips."
                ) {
                    Toggle("Use provider accent colors", isOn: $useProviderAccentColors)
                }
            }
            .padding(.vertical, 12)
            .frame(maxWidth: .infinity, alignment: .leading)
        }
    }
}
