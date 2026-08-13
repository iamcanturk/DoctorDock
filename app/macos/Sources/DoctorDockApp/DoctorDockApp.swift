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
///
/// The obvious implementation — start a Task, block on a semaphore — deadlocks.
/// The self-test drives a ScanStore, which is @MainActor, and the MainActor's
/// work runs on this thread. Blocking it means that work can never execute, so
/// the test waits forever for something it has itself prevented from running.
///
/// Pumping the run loop instead keeps the main thread available while waiting,
/// which is the same reason a spinner in a GUI needs the main thread free.
private func runSelfTestSynchronously() -> Int32 {
    var result: Int32 = 1
    var finished = false

    Task { @MainActor in
        result = await SelfTest.run()
        finished = true
    }

    let deadline = Date().addingTimeInterval(180)
    while !finished && Date() < deadline {
        RunLoop.current.run(mode: .default, before: Date().addingTimeInterval(0.05))
    }

    if !finished {
        print("\nFAIL — the self-test did not finish within 180s\n")
        return 1
    }
    return result
}
