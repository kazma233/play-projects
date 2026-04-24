#if os(macOS)
import AppKit
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
    case help
    case usage
    case message(String)
}

func printUsage() {
    let usage = """
    Usage: make-rounded-icon-macos.swift <input> <output> [options]

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
      - This script is macOS-only and uses AppKit rendering.
    """

    fputs("\(usage)\n", stderr)
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

    if arguments.contains("--help") {
        throw ScriptError.help
    }

    if arguments.isEmpty {
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

do {
    let options = try parseOptions()
    let sourceImage = try loadSourceImage(from: options.inputURL)
    let pngData = try renderRoundedIcon(sourceImage: sourceImage, options: options)
    try writeOutput(pngData, to: options.outputURL)
} catch ScriptError.help {
    printUsage()
    exit(0)
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
#else
import Foundation
fputs("make-rounded-icon-macos.swift only supports macOS.\n", stderr)
exit(1)
#endif
