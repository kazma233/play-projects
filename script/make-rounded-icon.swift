import Foundation

enum ScriptError: Error {
    case unsupportedPlatform(String)
    case failedToStart(String)
    case childExited(Int32)
}

func targetScriptName() throws -> String {
    #if os(macOS)
    return "make-rounded-icon-macos.swift"
    #elseif os(Linux)
    return "make-rounded-icon-linux.swift"
    #else
    throw ScriptError.unsupportedPlatform("Unsupported platform. Use macOS or Linux.")
    #endif
}

func scriptDirectory() -> URL {
    URL(fileURLWithPath: #filePath).deletingLastPathComponent()
}

func run() throws {
    let targetURL = scriptDirectory().appendingPathComponent(try targetScriptName())

    guard FileManager.default.fileExists(atPath: targetURL.path) else {
        throw ScriptError.failedToStart("Platform script not found: \(targetURL.path)")
    }

    let process = Process()
    process.executableURL = URL(fileURLWithPath: "/usr/bin/env")
    process.arguments = ["swift", targetURL.path] + Array(CommandLine.arguments.dropFirst())
    process.standardInput = FileHandle.standardInput
    process.standardOutput = FileHandle.standardOutput
    process.standardError = FileHandle.standardError

    do {
        try process.run()
    } catch {
        throw ScriptError.failedToStart("Failed to start swift: \(error)")
    }

    process.waitUntilExit()

    guard process.terminationStatus == 0 else {
        throw ScriptError.childExited(process.terminationStatus)
    }
}

do {
    try run()
} catch ScriptError.unsupportedPlatform(let message) {
    fputs("\(message)\n", stderr)
    exit(1)
} catch ScriptError.failedToStart(let message) {
    fputs("\(message)\n", stderr)
    exit(1)
} catch ScriptError.childExited(let code) {
    exit(code)
} catch {
    fputs("Unexpected error: \(error)\n", stderr)
    exit(1)
}
