import SwiftUI

@main
struct DoctorDockApp: App {
    @StateObject private var store = ScanStore()

    var body: some Scene {
        // MenuBarExtra with .window gives a popover rather than a menu, which
        // is what allows real layout — a menu can only hold rows.
        MenuBarExtra {
            PopoverView()
                .environmentObject(store)
        } label: {
            MenuBarLabel(store: store)
        }
        .menuBarExtraStyle(.window)

        // The full panel is a separate window so it can be left open, resized
        // and tabbed through while the popover stays a glance.
        Window("DoctorDock", id: PanelWindow.id) {
            PanelView()
                .environmentObject(store)
                .frame(minWidth: 820, minHeight: 520)
        }
        .defaultSize(width: 980, height: 660)
        .commands {
            CommandGroup(replacing: .newItem) {}
            CommandGroup(after: .toolbar) {
                Button("Refresh") {
                    Task { await store.refresh() }
                }
                .keyboardShortcut("r", modifiers: .command)
            }
        }

        Settings {
            SettingsView()
                .environmentObject(store)
        }
    }

    init() {
        // `--selftest` exercises the whole Swift-to-Go path and exits without
        // ever creating a window, so the bridge can be checked from a script.
        if CommandLine.arguments.contains("--selftest") {
            let code = runSelfTestSynchronously()
            exit(code)
        }

        // The app has no Dock icon and no main menu; LSUIElement in Info.plist
        // declares that, and this makes it true when running from `swift run`
        // where there is no bundle.
        NSApplication.shared.setActivationPolicy(.accessory)
    }
}

enum PanelWindow {
    static let id = "doctordock-panel"
}

/// Bridges the async self-test into `init`, which cannot await.
private func runSelfTestSynchronously() -> Int32 {
    let semaphore = DispatchSemaphore(value: 0)
    var result: Int32 = 1
    Task {
        result = await SelfTest.run()
        semaphore.signal()
    }
    semaphore.wait()
    return result
}
