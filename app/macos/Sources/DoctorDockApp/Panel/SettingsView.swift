import SwiftUI

struct SettingsView: View {
    @EnvironmentObject private var store: ScanStore

    var body: some View {
        Form {
            Section("Scanning") {
                Picker("Refresh every", selection: Binding(
                    get: { store.refreshInterval },
                    set: { store.refreshInterval = $0; store.scheduleTimer() }
                )) {
                    Text("1 minute").tag(TimeInterval(60))
                    Text("5 minutes").tag(TimeInterval(300))
                    Text("15 minutes").tag(TimeInterval(900))
                    Text("1 hour").tag(TimeInterval(3600))
                    Text("Never").tag(TimeInterval(0))
                }
                Text("A scan takes under a second and only reads from the Docker API. It never changes anything.")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }

            Section("Notifications") {
                Toggle("When the score drops", isOn: $store.notifyOnScoreDrop)
                Toggle("When a new critical finding appears", isOn: $store.notifyOnNewCritical)
                Text("Only new findings notify. A problem you have chosen to live with will not tell you about itself every five minutes.")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }

            Section("About") {
                LabeledContent("Engine", value: store.binaryVersion.map { "doctordock \($0)" } ?? "not found")
                if let report = store.report {
                    LabeledContent("Docker", value: report.docker.display)
                    LabeledContent("Report schema", value: report.schemaVersion)
                }
                Link("github.com/iamcanturk/DoctorDock",
                     destination: URL(string: "https://github.com/iamcanturk/DoctorDock")!)

                // A menubar app has no console. When something goes wrong the
                // only way to find out what is this file, so it is offered
                // rather than left to be discovered.
                HStack {
                    Text("Diagnostics")
                    Spacer()
                    Button("Reveal log") {
                        NSWorkspace.shared.selectFile(Log.path, inFileViewerRootedAtPath: "")
                    }
                    .controlSize(.small)
                }
            }
        }
        .formStyle(.grouped)
        .frame(width: 460, height: 420)
    }
}
