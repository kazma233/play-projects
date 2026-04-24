#if canImport(AppKit)
import AppKit
#endif
import Foundation

struct Options {
    let inputURL: URL
    let outputURL: URL
    let canvasSize: CGFloat
    let inset: CGFloat
    let cornerRadius: CGFloat
    let shadowEnabled: Bool
    let shadowAlpha: CGFloat
    let shadowBlur: CGFloat
    let shadowOffsetX: CGFloat
    let shadowOffsetY: CGFloat
}

enum ScriptError: Error {
    case usage
    case message(String)
}

func printUsage() {
    let usage = """
    Usage: make-rounded-icon.swift <input> <output> [options]

    Options:
      --canvas <size>            Output canvas size in pixels. Default: 1024
      --inset <size>             Transparent margin around the icon. Default: 96
      --corner-radius <size>     Rounded corner radius. Default: 220
      --shadow <on|off>          Enable or disable the outer shadow. Default: on
      --shadow-alpha <value>     Shadow opacity from 0 to 1. Default: 0.22
      --shadow-blur <size>       Shadow blur radius. Default: 32
      --shadow-offset-x <size>   Horizontal shadow offset. Default: 0
      --shadow-offset-y <size>   Vertical shadow offset. Default: -10
      --help                     Show this help message

    Notes:
      - Non-square source images are center-cropped to a square automatically.
      - The output stays square and keeps a transparent border around the rounded icon.
      - On Linux, ImageMagick (`magick`) is required.
    """

    fputs("\(usage)\n", stderr)
}

func formatCGFloat(_ value: CGFloat) -> String {
    if value.rounded(.towardZero) == value {
        return String(Int(value))
    }
    return String(format: "%.3f", Double(value))
}

func formatSignedCGFloat(_ value: CGFloat) -> String {
    let formatted = formatCGFloat(value.magnitude)
    return value >= 0 ? "+\(formatted)" : "-\(formatted)"
}

func parseCGFloat(_ value: String, flag: String) throws -> CGFloat {
    guard let parsed = Double(value) else {
        throw ScriptError.message("Invalid value for \(flag): \(value)")
    }
    return CGFloat(parsed)
}

func parseBool(_ value: String, flag: String) throws -> Bool {
    switch value.lowercased() {
    case "on", "true", "1", "yes":
        return true
    case "off", "false", "0", "no":
        return false
    default:
        throw ScriptError.message("Invalid value for \(flag): \(value). Use on/off.")
    }
}

func requireValue(for flag: String, in arguments: [String], at index: inout Int) throws -> String {
    index += 1
    guard index < arguments.count else {
        throw ScriptError.message("Missing value for \(flag)")
    }
    return arguments[index]
}

func parseOptions() throws -> Options {
    let arguments = Array(CommandLine.arguments.dropFirst())

    if arguments.isEmpty || arguments.contains("--help") {
        throw ScriptError.usage
    }

    guard arguments.count >= 2 else {
        throw ScriptError.usage
    }

    let inputURL = URL(fileURLWithPath: arguments[0])
    let outputURL = URL(fileURLWithPath: arguments[1])

    var canvasSize: CGFloat = 1024
    var inset: CGFloat = 96
    var cornerRadius: CGFloat = 220
    var shadowEnabled = true
    var shadowAlpha: CGFloat = 0.22
    var shadowBlur: CGFloat = 32
    var shadowOffsetX: CGFloat = 0
    var shadowOffsetY: CGFloat = -10

    var index = 2
    while index < arguments.count {
        let flag = arguments[index]

        switch flag {
        case "--canvas":
            canvasSize = try parseCGFloat(try requireValue(for: flag, in: arguments, at: &index), flag: flag)
        case "--inset":
            inset = try parseCGFloat(try requireValue(for: flag, in: arguments, at: &index), flag: flag)
        case "--corner-radius":
            cornerRadius = try parseCGFloat(try requireValue(for: flag, in: arguments, at: &index), flag: flag)
        case "--shadow":
            shadowEnabled = try parseBool(try requireValue(for: flag, in: arguments, at: &index), flag: flag)
        case "--shadow-alpha":
            shadowAlpha = try parseCGFloat(try requireValue(for: flag, in: arguments, at: &index), flag: flag)
        case "--shadow-blur":
            shadowBlur = try parseCGFloat(try requireValue(for: flag, in: arguments, at: &index), flag: flag)
        case "--shadow-offset-x":
            shadowOffsetX = try parseCGFloat(try requireValue(for: flag, in: arguments, at: &index), flag: flag)
        case "--shadow-offset-y":
            shadowOffsetY = try parseCGFloat(try requireValue(for: flag, in: arguments, at: &index), flag: flag)
        default:
            throw ScriptError.message("Unknown option: \(flag)")
        }

        index += 1
    }

    guard canvasSize > 0 else {
        throw ScriptError.message("--canvas must be greater than 0")
    }

    guard inset >= 0 else {
        throw ScriptError.message("--inset must be 0 or greater")
    }

    let drawableSize = canvasSize - inset * 2
    guard drawableSize > 0 else {
        throw ScriptError.message("--inset is too large for the selected canvas size")
    }

    guard cornerRadius >= 0 else {
        throw ScriptError.message("--corner-radius must be 0 or greater")
    }

    guard cornerRadius <= drawableSize / 2 else {
        throw ScriptError.message("--corner-radius must be no larger than half of the inner size (\(Int(drawableSize / 2)))")
    }

    guard shadowAlpha >= 0 && shadowAlpha <= 1 else {
        throw ScriptError.message("--shadow-alpha must be between 0 and 1")
    }

    guard shadowBlur >= 0 else {
        throw ScriptError.message("--shadow-blur must be 0 or greater")
    }

    return Options(
        inputURL: inputURL,
        outputURL: outputURL,
        canvasSize: canvasSize,
        inset: inset,
        cornerRadius: cornerRadius,
        shadowEnabled: shadowEnabled,
        shadowAlpha: shadowAlpha,
        shadowBlur: shadowBlur,
        shadowOffsetX: shadowOffsetX,
        shadowOffsetY: shadowOffsetY
    )
}

func centeredSquareCropRect(for size: CGSize) -> CGRect {
    let edge = min(size.width, size.height)
    return CGRect(
        x: (size.width - edge) / 2,
        y: (size.height - edge) / 2,
        width: edge,
        height: edge
    )
}

func prepareOutputDirectory(for url: URL) throws {
    do {
        try FileManager.default.createDirectory(
            at: url.deletingLastPathComponent(),
            withIntermediateDirectories: true
        )
    } catch {
        throw ScriptError.message("Failed to prepare output directory: \(error)")
    }
}

func runCommand(_ program: String, arguments: [String]) throws -> Data {
    let process = Process()
    let stdoutPipe = Pipe()
    let stderrPipe = Pipe()

    process.executableURL = URL(fileURLWithPath: "/usr/bin/env")
    process.arguments = [program] + arguments
    process.standardOutput = stdoutPipe
    process.standardError = stderrPipe

    do {
        try process.run()
    } catch {
        throw ScriptError.message("Failed to start \(program): \(error)")
    }

    process.waitUntilExit()

    let stdout = stdoutPipe.fileHandleForReading.readDataToEndOfFile()
    let stderr = stderrPipe.fileHandleForReading.readDataToEndOfFile()

    guard process.terminationStatus == 0 else {
        let message = String(data: stderr, encoding: .utf8)?
            .trimmingCharacters(in: .whitespacesAndNewlines)
        throw ScriptError.message(message?.isEmpty == false ? message! : "\(program) exited with code \(process.terminationStatus)")
    }

    return stdout
}

#if canImport(AppKit)
func loadSourceImage(from url: URL) throws -> NSImage {
    guard let sourceImage = NSImage(contentsOf: url) else {
        throw ScriptError.message("Unable to load source image: \(url.path)")
    }
    return sourceImage
}

func sourcePixelSize(for image: NSImage) -> CGSize {
    if let rep = image.representations.first(where: { $0.pixelsWide > 0 && $0.pixelsHigh > 0 }) {
        return CGSize(width: rep.pixelsWide, height: rep.pixelsHigh)
    }

    return image.size
}

func validateSourceImage(_ image: NSImage, sourceURL: URL) throws -> CGSize {
    let pixelSize = sourcePixelSize(for: image)
    guard pixelSize.width > 0, pixelSize.height > 0 else {
        throw ScriptError.message("Unable to determine image size: \(sourceURL.path)")
    }
    return pixelSize
}

func renderRoundedIcon(sourceImage: NSImage, options: Options) throws -> Data {
    let sourceSize = try validateSourceImage(sourceImage, sourceURL: options.inputURL)
    sourceImage.size = sourceSize

    let canvasSize = CGSize(width: options.canvasSize, height: options.canvasSize)
    let targetRect = CGRect(
        x: options.inset,
        y: options.inset,
        width: options.canvasSize - options.inset * 2,
        height: options.canvasSize - options.inset * 2
    )
    let sourceCropRect = centeredSquareCropRect(for: sourceSize)

    guard
        let bitmap = NSBitmapImageRep(
            bitmapDataPlanes: nil,
            pixelsWide: Int(options.canvasSize),
            pixelsHigh: Int(options.canvasSize),
            bitsPerSample: 8,
            samplesPerPixel: 4,
            hasAlpha: true,
            isPlanar: false,
            colorSpaceName: .deviceRGB,
            bytesPerRow: 0,
            bitsPerPixel: 0
        ),
        let graphicsContext = NSGraphicsContext(bitmapImageRep: bitmap)
    else {
        throw ScriptError.message("Failed to create bitmap context")
    }

    NSGraphicsContext.saveGraphicsState()
    NSGraphicsContext.current = graphicsContext
    defer {
        NSGraphicsContext.restoreGraphicsState()
    }

    graphicsContext.imageInterpolation = .high

    NSColor.clear.setFill()
    NSRect(origin: .zero, size: canvasSize).fill()

    if options.shadowEnabled {
        NSGraphicsContext.current?.saveGraphicsState()

        let shadow = NSShadow()
        shadow.shadowColor = NSColor(calibratedWhite: 0, alpha: options.shadowAlpha)
        shadow.shadowBlurRadius = options.shadowBlur
        shadow.shadowOffset = CGSize(width: options.shadowOffsetX, height: options.shadowOffsetY)
        shadow.set()

        let shadowPath = NSBezierPath(
            roundedRect: targetRect,
            xRadius: options.cornerRadius,
            yRadius: options.cornerRadius
        )
        NSColor(calibratedWhite: 1, alpha: 0.001).setFill()
        shadowPath.fill()

        NSGraphicsContext.current?.restoreGraphicsState()
    }

    let clipPath = NSBezierPath(
        roundedRect: targetRect,
        xRadius: options.cornerRadius,
        yRadius: options.cornerRadius
    )
    clipPath.addClip()

    sourceImage.draw(
        in: targetRect,
        from: sourceCropRect,
        operation: .sourceOver,
        fraction: 1.0,
        respectFlipped: false,
        hints: [.interpolation: NSImageInterpolation.high]
    )

    guard let pngData = bitmap.representation(using: .png, properties: [:]) else {
        throw ScriptError.message("Failed to encode PNG output")
    }

    return pngData
}

func writeOutput(_ data: Data, to url: URL) throws {
    do {
        try prepareOutputDirectory(for: url)
        try data.write(to: url)
    } catch {
        throw ScriptError.message("Failed to write output PNG: \(error)")
    }
}

func processIcon(options: Options) throws {
    let sourceImage = try loadSourceImage(from: options.inputURL)
    let pngData = try renderRoundedIcon(sourceImage: sourceImage, options: options)
    try writeOutput(pngData, to: options.outputURL)
}
#else
func loadSourceImageDimensions(from url: URL) throws -> CGSize {
    let output = try runCommand(
        "magick",
        arguments: ["identify", "-format", "%w %h", url.path]
    )

    guard
        let raw = String(data: output, encoding: .utf8)?
            .trimmingCharacters(in: .whitespacesAndNewlines),
        !raw.isEmpty
    else {
        throw ScriptError.message("Unable to determine image size: \(url.path)")
    }

    let parts = raw.split(separator: " ")
    guard
        parts.count == 2,
        let width = Double(parts[0]),
        let height = Double(parts[1]),
        width > 0,
        height > 0
    else {
        throw ScriptError.message("Unable to determine image size: \(url.path)")
    }

    return CGSize(width: width, height: height)
}

func processIcon(options: Options) throws {
    let sourceSize = try loadSourceImageDimensions(from: options.inputURL)
    let cropRect = centeredSquareCropRect(for: sourceSize)
    let canvasPixels = Int(options.canvasSize)
    let drawablePixels = Int(options.canvasSize - options.inset * 2)

    guard canvasPixels > 0, drawablePixels > 0 else {
        throw ScriptError.message("Canvas size is invalid for ImageMagick rendering")
    }

    try prepareOutputDirectory(for: options.outputURL)

    let fileManager = FileManager.default
    let tempDirectory = fileManager.temporaryDirectory
        .appendingPathComponent("make-rounded-icon-\(UUID().uuidString)", isDirectory: true)

    do {
        try fileManager.createDirectory(at: tempDirectory, withIntermediateDirectories: true)
    } catch {
        throw ScriptError.message("Failed to create temporary directory: \(error)")
    }

    defer {
        try? fileManager.removeItem(at: tempDirectory)
    }

    let squareURL = tempDirectory.appendingPathComponent("square.png")
    let maskURL = tempDirectory.appendingPathComponent("mask.png")
    let roundedURL = tempDirectory.appendingPathComponent("rounded.png")
    let canvasURL = tempDirectory.appendingPathComponent("canvas.png")
    let shadowedURL = tempDirectory.appendingPathComponent("shadowed.png")

    _ = try runCommand(
        "magick",
        arguments: [
            options.inputURL.path,
            "-crop", "\(formatCGFloat(cropRect.width))x\(formatCGFloat(cropRect.height))+\(formatCGFloat(cropRect.origin.x))+\(formatCGFloat(cropRect.origin.y))",
            "+repage",
            "-resize", "\(drawablePixels)x\(drawablePixels)!",
            squareURL.path
        ]
    )

    _ = try runCommand(
        "magick",
        arguments: [
            "-size", "\(drawablePixels)x\(drawablePixels)",
            "xc:black",
            "-fill", "white",
            "-draw", "roundrectangle 0,0,\(drawablePixels - 1),\(drawablePixels - 1),\(formatCGFloat(options.cornerRadius)),\(formatCGFloat(options.cornerRadius))",
            maskURL.path
        ]
    )

    _ = try runCommand(
        "magick",
        arguments: [
            squareURL.path,
            maskURL.path,
            "-alpha", "off",
            "-compose", "CopyOpacity",
            "-composite",
            roundedURL.path
        ]
    )

    _ = try runCommand(
        "magick",
        arguments: [
            "-size", "\(canvasPixels)x\(canvasPixels)",
            "xc:none",
            roundedURL.path,
            "-gravity", "center",
            "-composite",
            canvasURL.path
        ]
    )

    if options.shadowEnabled {
        let shadowOpacity = Int((options.shadowAlpha * 100).rounded())
        _ = try runCommand(
            "magick",
            arguments: [
                canvasURL.path,
                "(",
                "+clone",
                "-background", "black",
                "-shadow", "\(shadowOpacity)x\(formatCGFloat(options.shadowBlur))\(formatSignedCGFloat(options.shadowOffsetX))\(formatSignedCGFloat(options.shadowOffsetY))",
                ")",
                "+swap",
                "-background", "none",
                "-layers", "merge",
                "+repage",
                shadowedURL.path
            ]
        )

        _ = try runCommand(
            "magick",
            arguments: [
                shadowedURL.path,
                "-background", "none",
                "-gravity", "center",
                "-extent", "\(canvasPixels)x\(canvasPixels)",
                options.outputURL.path
            ]
        )
    } else {
        _ = try runCommand(
            "magick",
            arguments: [canvasURL.path, options.outputURL.path]
        )
    }
}
#endif

do {
    let options = try parseOptions()
    try processIcon(options: options)
} catch ScriptError.usage {
    printUsage()
    exit(1)
} catch ScriptError.message(let message) {
    fputs("\(message)\n", stderr)
    exit(1)
} catch {
    fputs("Unexpected error: \(error)\n", stderr)
    exit(1)
}
