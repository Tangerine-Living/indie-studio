"""Generate the PixelBench app icons from the same 5x5 pixel mark used in the UI.

Outputs:
  build/windows/icon.ico   16/32/48/64/128/256 px, uncompressed 32-bit BMP
                           entries (NSIS cannot read PNG-compressed icons)
  build/appicon.png        1024x1024 RGBA

The glyph mirrors frontend/src/components/PixelMark.vue:
  a = amber stem / bowl     c = green fill     . = transparent
Run: python scripts/make-icon.py
"""

import os
import struct

from PIL import Image, ImageDraw

BG = (26, 18, 5, 255)
BORDER = (91, 68, 26, 255)
AMBER = (242, 184, 75, 255)
GREEN = (110, 231, 168, 255)
CLEAR = (0, 0, 0, 0)

PATTERN = ["accca", "a...a", "a...a", "accca", "a...."]
GLYPH_SCALE = 2  # 5 cells x 2 px = 10px glyph inside a 16px tile
GLYPH_ORIGIN = 3

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))


def render(scale: int) -> Image.Image:
    """Render the 16x16 design at an integer scale."""
    size = 16 * scale
    im = Image.new("RGBA", (size, size), CLEAR)
    draw = ImageDraw.Draw(im)

    def px(x: int, y: int, color) -> None:
        draw.rectangle(
            [x * scale, y * scale, (x + 1) * scale - 1, (y + 1) * scale - 1],
            fill=color,
        )

    for y in range(16):
        for x in range(16):
            if x in (0, 15) and y in (0, 15):
                continue
            px(x, y, BG)

    for x in range(1, 15):
        px(x, 0, BORDER)
        px(x, 15, BORDER)
    for y in range(1, 15):
        px(0, y, BORDER)
        px(15, y, BORDER)

    for row, chars in enumerate(PATTERN):
        for col, char in enumerate(chars):
            if char == ".":
                continue
            color = AMBER if char == "a" else GREEN
            gx = GLYPH_ORIGIN + col * GLYPH_SCALE
            gy = GLYPH_ORIGIN + row * GLYPH_SCALE
            for dy in range(GLYPH_SCALE):
                for dx in range(GLYPH_SCALE):
                    px(gx + dx, gy + dy, color)

    return im


def ico_blob(images):
    """Encode images as an ICO with plain BMP entries (NSIS-compatible)."""
    entries = []
    blobs = []
    offset = 6 + 16 * len(images)
    for im in images:
        w, h = im.size
        pixels = im.load()
        rows = []
        for y in range(h - 1, -1, -1):
            row = bytearray()
            for x in range(w):
                r, g, b, a = pixels[x, y]
                row += bytes((b, g, r, a))
            rows.append(bytes(row))
        mask_stride = ((w + 31) // 32) * 4
        mask = b"\x00" * (mask_stride * h)
        bitmap = b"".join(rows)
        dib = struct.pack(
            "<IiiHHIIiiII",
            40,        # biSize
            w,         # biWidth
            h * 2,     # biHeight: XOR + AND mask
            1,         # biPlanes
            32,        # biBitCount
            0,         # biCompression = BI_RGB
            len(bitmap) + len(mask),  # biSizeImage
            0, 0, 0, 0,
        ) + bitmap + mask
        entries.append(
            struct.pack(
                "<BBBBHHII",
                w % 256, h % 256, 0, 0, 1, 32, len(dib), offset,
            )
        )
        blobs.append(dib)
        offset += len(dib)
    return struct.pack("<HHH", 0, 1, len(images)) + b"".join(entries) + b"".join(blobs)


def main() -> None:
    sizes = [16, 32, 48, 64, 128, 256]
    images = [render(size // 16) for size in sizes]

    ico_path = os.path.join(ROOT, "build", "windows", "icon.ico")
    with open(ico_path, "wb") as fh:
        fh.write(ico_blob(images))
    print(f"wrote {ico_path} ({os.path.getsize(ico_path)} bytes, {sizes})")

    png_path = os.path.join(ROOT, "build", "appicon.png")
    render(64).save(png_path)
    print(f"wrote {png_path} (1024x1024)")


if __name__ == "__main__":
    main()