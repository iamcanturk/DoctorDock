import Foundation

// The structured explanation behind a finding, from `doctordock explain
// <id> --format json`. Rendering it natively — sections, code blocks, links —
// is what replaced dumping the CLI's terminal text into the panel.

struct RuleExplanation: Codable {
    let id: String
    let name: String
    let severity: Severity
    let category: Category
    let description: String
    let explanation: Explanation
    let hasLongForm: Bool

    struct Explanation: Codable {
        let what: String
        let why: String
        let scenario: String?
        let fixes: [Fix]
        let falsePositives: String?
        let references: [Reference]?
    }

    struct Fix: Codable, Identifiable {
        let title: String
        let lang: String
        let code: String
        var id: String { title }
    }

    struct Reference: Codable, Identifiable {
        let title: String
        let url: String
        var id: String { url }
    }
}
