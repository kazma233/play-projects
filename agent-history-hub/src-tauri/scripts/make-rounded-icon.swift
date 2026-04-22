import AppKit
import Foundation

let arguments = CommandLine.arguments

guard arguments.count == 3 else {
    fputs("Usage: make-rounded-icon.swift <input> <output>\n", stderr)
    exit(1)
}

let inputURL = URL(fileURLWithPath: arguments[1])
let outputURL = URL(fileURLWithPath: arguments[2])

guard let sourceImage = NSImage(contentsOf: inputURL) else {
    fputs("Unable to load source image: \(inputURL.path)\n", stderr)
    exit(1)
}

let canvasSize = CGSize(width: 1024, height: 1024)
let inset: CGFloat = 96
let targetRect = CGRect(
    x: inset,
    y: inset,
    width: canvasSize.width - inset * 2,
    height: canvasSize.height - inset * 2
)
let cornerRadius: CGFloat = 220

let outputImage = NSImage(size: canvasSize)
outputImage.lockFocus()

NSColor.clear.setFill()
NSRect(origin: .zero, size: canvasSize).fill()

let shadow = NSShadow()
shadow.shadowColor = NSColor(calibratedWhite: 0, alpha: 0.22)
shadow.shadowBlurRadius = 32
shadow.shadowOffset = CGSize(width: 0, height: -10)
shadow.set()

let clipPath = NSBezierPath(roundedRect: targetRect, xRadius: cornerRadius, yRadius: cornerRadius)
clipPath.addClip()

sourceImage.draw(
    in: targetRect,
    from: NSRect(origin: .zero, size: sourceImage.size),
    operation: .sourceOver,
    fraction: 1.0,
    respectFlipped: false,
    hints: [.interpolation: NSImageInterpolation.high]
)

outputImage.unlockFocus()

guard
    let tiffData = outputImage.tiffRepresentation,
    let bitmap = NSBitmapImageRep(data: tiffData),
    let pngData = bitmap.representation(using: .png, properties: [:])
else {
    fputs("Failed to encode PNG output\n", stderr)
    exit(1)
}

try FileManager.default.createDirectory(
    at: outputURL.deletingLastPathComponent(),
    withIntermediateDirectories: true
)

do {
    try pngData.write(to: outputURL)
} catch {
    fputs("Failed to write output PNG: \(error)\n", stderr)
    exit(1)
}
